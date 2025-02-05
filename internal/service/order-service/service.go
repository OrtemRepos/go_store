package orderservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/OrtemRepos/go_store/internal/domain"
	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/OrtemRepos/go_store/internal/worker-pool"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrInternalServerError = errors.New("internal server error")
	ErrRequestTimeout = errors.New("timeout request")
	ErrGatewayTimeout = errors.New("timeout gateway")
	ErrNotFound = errors.New("order not found")
	ErrMaxRetry = errors.New("max retries exceeded")
	ErrBufferFull = errors.New("buffer full")
)

type RetryableError struct {
	RetryAfter time.Duration
	Message    string
}

func (re *RetryableError) Error() string {
	return fmt.Sprintf("msg: %s, timeout: %v", re.Message, re.RetryAfter)
}

type client struct {
	BaseURL    string
	HTTPClient *http.Client
	MaxRetries int
	RetryDelay time.Duration
	logger     *zap.Logger
}

func newClient(baseURL string, maxRetries, retryDelay int, logger *zap.Logger) *client {
	return &client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: time.Second * 10},
		MaxRetries: maxRetries,
		RetryDelay: time.Millisecond * time.Duration(retryDelay),
		logger: logger,
	}
}

func NewOrderService(db *gorm.DB, logger *zap.Logger, wp worker.WorkerPool, userStorage ports.UserStorage, accuralAddress string, maxRetries, retryDelay int) (*OrderService, error) {
	client := newClient(accuralAddress, maxRetries, retryDelay, logger)
	if wp == nil {
		return nil, fmt.Errorf("WorkerPool[worker.WorkerPool] is a mandatory dependency")
	}
	if db == nil {
		return nil, fmt.Errorf("db[gorm.DB] is a mandatory dependency")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger[zap.Logger] is a mandatory dependency")
	}
	if accuralAddress == "" {
		return nil, fmt.Errorf("accuralAddress[string] must not be nil or an empty string")
	}
	if maxRetries < 0 {
		return nil, fmt.Errorf("maxRetries[int] must be a non-negative number")
	}
	if retryDelay <= 0 {
		return nil, fmt.Errorf("retryDelay[int] must be greater than zero")
	}
	if userStorage == nil {
		return nil, fmt.Errorf("userStorage[ports.UserStorage] is a mandatory dependency")
	}

	os := &OrderService{
		db: db,
		logger: logger,
		client: *client,
		wp: wp,
		userStorage: userStorage,
	}
	return os, nil
}

func (c *client) getOrderInfo(ctx context.Context, orderNumber string) (*domain.Order, error) {
	url := fmt.Sprintf("http://%s/api/orders/%s", c.BaseURL, orderNumber)

	var order *domain.Order
	var err error
	retryDelay := c.RetryDelay
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		order, err = c.doRequest(ctx, url)
		if err == nil {
			return order, nil
		}
		var retrErr *RetryableError
		if errors.As(err, &retrErr) {
			retryDelay = retrErr.RetryAfter
		}

		if !shouldRetry(err) {
			return nil, err
		}

		c.logger.Info("retry", zap.String("url", url), zap.Int("attempt", attempt), zap.Error(err))

		select {
		case <- time.After(retryDelay):
			retryDelay = c.RetryDelay
		case <- ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("maximum number of repeated requests: %w", ErrMaxRetry)
}

func (c *client) doRequest(ctx context.Context, url string) (*domain.Order, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var order domain.Order
		if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
			return nil, err
		}
		return &order, nil
	case http.StatusTooManyRequests:
		retryAfterStr := resp.Header.Get("Retry-After")
		var retryAfter time.Duration
		if sec, err := strconv.Atoi(retryAfterStr); err == nil {
			retryAfter = time.Duration(sec) * time.Second
		} else if date, err := http.ParseTime(retryAfterStr); err == nil {
			retryAfter = time.Until(date)
		} else {
			c.logger.Warn(
				"invalid Retry-After header",
				zap.String("value", retryAfterStr),
				zap.Error(err),
			)
			retryAfter = 60 * time.Second // Default to 60s
		}
		return nil, &RetryableError{
			RetryAfter: time.Duration(retryAfter),
			Message:    "too many requests",
		}
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("accural service error: %w", ErrInternalServerError)
	case http.StatusRequestTimeout:
		return nil, ErrRequestTimeout
	case http.StatusGatewayTimeout:
		return nil, ErrGatewayTimeout
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}


func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	var retrErr *RetryableError
	if errors.As(err, &retrErr) {
		return true
	}

	netErr, ok := err.(net.Error)
	if ok && netErr.Timeout() {
		return true
	}
	
	return errors.Is(err, ErrInternalServerError) ||
		errors.Is(err, ErrRequestTimeout) ||
		errors.Is(err, ErrGatewayTimeout)
}


type OrderService struct {
	db          *gorm.DB
	userStorage ports.UserStorage
	logger      *zap.Logger
	client      client
	wp          worker.WorkerPool
	orderResult    chan domain.Order
}

func (os *OrderService) Metrics() worker.MetricsResult {
	return os.wp.Metrics()
}

func (os *OrderService) Start(ctx context.Context) {
	os.wp.Start(ctx)
}

func (os *OrderService) ProcessOrder(ctx context.Context, order domain.Order) (*domain.Order, error) {
	var attempt = 0
	var delay = os.client.RetryDelay
	return os.processOrder(ctx, order, attempt, int(delay))
}

func (os *OrderService) processOrder(ctx context.Context, order domain.Order, attempt, delay int) (*domain.Order, error) {
	os.logger.Info("start processing the order", zap.String("number_order", order.Number))
	remoteOrder, err := os.client.getOrderInfo(ctx, order.Number)
	if err != nil {
		os.logger.Info("error whan get order from accural system", zap.Error(err))
		if attempt < os.client.MaxRetries {
			time.Sleep(os.client.RetryDelay)
			return os.processOrder(ctx, order, attempt+1, delay*2)
		}
		return nil, err
	}
	remoteOrder.UserID = order.UserID
	os.logger.Debug("got order", zap.Any("order", remoteOrder))
	if remoteOrder.Status == domain.INVALID {
		remoteOrder.Completed = true
		err = os.db.Model(&domain.Order{}).Where("number = ?", order.Number).Updates(remoteOrder).Error
		if err != nil {
			os.logger.Warn("error when saving invalid order", zap.Error(err))
			if attempt < os.client.MaxRetries {
				time.Sleep(time.Duration(delay))
				return os.processOrder(ctx, order, attempt+1, delay*2)
			}
			return nil, errors.Join(err, ErrMaxRetry)
		}
		return remoteOrder, nil
	} else if remoteOrder.Status == domain.PROCESSED {
		remoteOrder.Completed = true
		err = os.db.Model(&domain.Order{}).Where("number = ?", remoteOrder.Number).Updates(remoteOrder).Error
		if err != nil {
			os.logger.Warn("error when saving processed order", zap.Error(err))
			if attempt < os.client.MaxRetries {
				time.Sleep(time.Duration(delay))
				return os.processOrder(ctx, order, attempt+1, delay)
			}
			return nil, errors.Join(err, ErrMaxRetry)
		}
		os.logger.Debug("", zap.Int("accural", *remoteOrder.Accural))
		err = os.userStorage.AddAccural(remoteOrder.UserID, *remoteOrder.Accural)
		if err != nil {
			os.logger.Warn("error when add accural", zap.Error(err))
		}
		return remoteOrder, nil
	}
	if attempt < os.client.MaxRetries {
		time.Sleep(time.Duration(delay))
		return os.processOrder(ctx, order, attempt+1, delay)
	}
	return nil, ErrMaxRetry
}

type ProcessingOrderTask struct {
	os      *OrderService
	order   domain.Order
}

func (pt *ProcessingOrderTask) Execute(ctx context.Context) error {
	_, err := pt.os.ProcessOrder(ctx, pt.order)
	return err
}

func (pr *ProcessingOrderTask) Stringer() string {
	str := fmt.Sprintf("ProcessOrder: Order-%s", pr.order.Number)
	return str
}

func (os *OrderService) newTask(order domain.Order) ProcessingOrderTask {
	return ProcessingOrderTask{
		os: os,
		order: order,
	}
}

func (os *OrderService) AsyncProcessOrder(ctx context.Context, order domain.Order) error {
	task := os.newTask(order)
	if err := os.wp.Submit(ctx, &task); err != nil {
		os.logger.Error("failed to submit task", zap.Any("task", task), zap.Error(err))
		return err
	}
	return nil
}
