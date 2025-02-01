package configs

import (
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Database struct {
		Host     string `yaml:"host" env:"DB_HOST" env-description:"Database host-address"`
		Port     string `yaml:"port" env:"DB_PORT" env-description:"Database port"`
		Dbname   string `yaml:"dbname" env:"DB_NAME" env-description:"Database name"`
		User     string `yaml:"user" env:"DB_USER" env-description:"Database user"`
		Password string `yaml:"password" env:"DB_PASSWORD" env-description:"Database password"`
	} `yaml:"database"`
	Auth struct {
		TokenExp          int    `yaml:"tokenExp" env:"TOKEN_EXP" env-description:"Expire time for token"`
		SecretKey         string `yaml:"secretKey" env:"SECRET_KEY" env-description:"Secret key for token"`
		PasswordSecretKey string `yaml:"passwordSecretKey" env:"PASSWORD_SECRET_KEY" env-description:"Secret key for password"`
	} `yaml:"auth"`
}

type argsCommandLine struct {
	ConfigPath        string
	Host              string
	DatabasePort      string
	Dbname            string
	DatabaseUser      string
	DatabasePassword  string
	TokenExp          int
	SecretKey         string
	PasswordSecretKey string
}

func processArgs(argsToParse []string) (*argsCommandLine, map[string]bool, error) {
	a := new(argsCommandLine)
	f := flag.NewFlagSet("shortlink", flag.ContinueOnError)

	f.StringVar(&a.ConfigPath, "c",
		"/home/ortem917/go/src/github.com/OrtemRepos/go_store/configs/config.yml",
		"Path to configuration file")
	f.StringVar(&a.Host, "db-address", "", "Database host-address")
	f.StringVar(&a.DatabasePort, "db-port", "", "Database port")
	f.StringVar(&a.Dbname, "db-name", "", "Database name")
	f.StringVar(&a.DatabaseUser, "db-user", "", "Database user")
	f.StringVar(&a.DatabasePassword, "db-password", "", "Database password")
	f.IntVar(&a.TokenExp, "t", 0, "Token expiration duration")
	f.StringVar(&a.SecretKey, "sk", "", "Secret key for token")
	f.StringVar(&a.PasswordSecretKey, "psk", "", "Secret key for password")

	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		f.PrintDefaults()
	}

	if err := f.Parse(argsToParse); err != nil {
		return nil, nil, err
	}

	setFlags := make(map[string]bool)
	f.Visit(func(fl *flag.Flag) {
		setFlags[fl.Name] = true
	})

	return a, setFlags, nil
}

func GetConfig(argsToParse []string) (*Config, error) {
	cfg := new(Config)

	args, setFlags, err := processArgs(argsToParse)
	if err != nil {
		return nil, err
	}

	if err := cleanenv.ReadConfig(args.ConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("config read error: %w", err)
	}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("env read error: %w", err)
	}

	if err := overrideConfig(cfg, args, setFlags); err != nil {
		return nil, fmt.Errorf("config override error: %w", err)
	}
	cfg.Auth.TokenExp *= int(time.Second)
	logConfig(cfg)
	return cfg, nil
}

var flagMapping = map[string]string{
	"db-address":  "Database.Host",
	"db-port":     "Database.Port",
	"db-name":     "Database.Dbname",
	"db-user":     "Database.User",
	"db-password": "Database.Password",
	"t":           "Auth.TokenExp",
	"sk":          "Auth.SecretKey",
	"psk":         "Auth.PasswordSecKey",
}

func overrideConfig(cfg *Config, args *argsCommandLine, setFlags map[string]bool) error {
	argsVal := reflect.ValueOf(args).Elem()
	cfgVal := reflect.ValueOf(cfg).Elem()

	for flagName := range setFlags {
		fieldPath, ok := flagMapping[flagName]
		if !ok {
			continue
		}

		field := argsVal.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, flagName)
		})

		if !field.IsValid() {
			continue
		}

		if err := setConfigValue(cfgVal, fieldPath, field); err != nil {
			return err
		}
	}
	return nil
}

func setConfigValue(cfgVal reflect.Value, path string, value reflect.Value) error {
	fields := strings.Split(path, ".")
	for i, fieldName := range fields {
		if cfgVal.Kind() == reflect.Ptr {
			cfgVal = cfgVal.Elem()
		}

		cfgField := cfgVal.FieldByName(fieldName)
		if !cfgField.IsValid() {
			return fmt.Errorf("invalid config field: %s", fieldName)
		}

		if i == len(fields)-1 {
			return setFieldValue(cfgField, value)
		}

		if cfgField.Kind() == reflect.Ptr {
			if cfgField.IsNil() {
				cfgField.Set(reflect.New(cfgField.Type().Elem()))
			}
			cfgVal = cfgField.Elem()
		} else {
			cfgVal = cfgField
		}
	}
	return nil
}

func setFieldValue(field, value reflect.Value) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field value")
	}

	if field.Type() == value.Type() {
		field.Set(value)
		return nil
	}

	val := value.Interface()

	switch field.Kind() { //nolint:exhaustive
	case reflect.String:
		field.SetString(fmt.Sprint(val))
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		strVal := fmt.Sprint(val)
		intVal, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %w", err)
		}
		field.SetInt(intVal)
		return nil

	case reflect.Bool:
		strVal := fmt.Sprint(val)
		boolVal, err := strconv.ParseBool(strVal)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %w", err)
		}
		field.SetBool(boolVal)
		return nil
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}
}

func logConfig(cfg *Config) {
	log.Println("Loaded configuration:")
	log.Printf("Database.Host: %s", cfg.Database.Host)
	log.Printf("Database.Port: %s", cfg.Database.Port)
	log.Printf("Database.Dbname: %s", cfg.Database.Dbname)
	log.Printf("Database.User: %s", cfg.Database.User)
	log.Printf("Auth.TokenExp: %v", cfg.Auth.TokenExp)
}
