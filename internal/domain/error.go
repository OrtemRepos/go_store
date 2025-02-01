package domain

import (
	"errors"
)

var ErrUserNotExist = errors.New("user does not exist")

var ErrInvalidOrderNubmer = errors.New("order number is invalid")

var ErrOrderAlreadyExistsForUser = errors.New("user has already added this order")

var ErrOrderConflict = errors.New("the order number has already been uploaded by another user")