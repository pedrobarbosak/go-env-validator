package envParser

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Tag is the struct tag key used to identify environment variable names.
	Tag = "env"
	// Separator is the default separator used for slice and map values.
	Separator = ";"
)

// Validator is an interface for validating structs after unmarshaling.
// Compatible with github.com/go-playground/validator.
type Validator interface {
	Struct(v interface{}) error
}

var (
	validator   Validator
	validatorMu sync.RWMutex
)

// SetValidator sets a validator to be called after unmarshaling.
// The validator is called with the struct pointer after all fields are set.
// This function is thread-safe.
func SetValidator(v Validator) {
	validatorMu.Lock()
	defer validatorMu.Unlock()
	validator = v
}

func getValidator() Validator {
	validatorMu.RLock()
	defer validatorMu.RUnlock()
	return validator
}

type tagField struct {
	Key       string
	Default   string
	Required  bool
	Separator string
}

// EnvironToMap converts a slice of environment variables in "KEY=value" format
// to a map. Returns ErrInvalidEnviron if any entry is malformed.
func EnvironToMap(env []string) (map[string]string, error) {
	m := make(map[string]string, len(env))
	for _, s := range env {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, ErrInvalidEnviron
		}

		m[parts[0]] = parts[1]
	}

	return m, nil
}

// UnmarshalFromEnv unmarshals environment variables from os.Environ() into v.
// v must be a non-nil pointer to a struct.
func UnmarshalFromEnv(v interface{}) error {
	envs, err := EnvironToMap(os.Environ())
	if err != nil {
		return err
	}

	return Unmarshal(envs, v)
}

// UnmarshalFromFile reads a .env file and unmarshals its contents into v,
// merged with the current system environment variables.
// File values take precedence over system environment variables.
// v must be a non-nil pointer to a struct.
func UnmarshalFromFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fileEnvs := parseEnvFile(string(data))
	fullEnvs := append(os.Environ(), fileEnvs...)

	envs, err := EnvironToMap(fullEnvs)
	if err != nil {
		return err
	}

	return Unmarshal(envs, v)
}

// UnmarshalFromFileOnly reads a .env file and unmarshals its contents into v.
// Unlike UnmarshalFromFile, this function ignores system environment variables.
// v must be a non-nil pointer to a struct.
func UnmarshalFromFileOnly(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fileEnvs := parseEnvFile(string(data))

	envs, err := EnvironToMap(fileEnvs)
	if err != nil {
		return err
	}

	return Unmarshal(envs, v)
}

func parseEnvFile(content string) []string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		result = append(result, line)
	}

	return result
}

// Unmarshal parses environment variables from a map into v.
// v must be a non-nil pointer to a struct.
// If a validator is set via SetValidator, it will be called after unmarshaling.
func Unmarshal(envs map[string]string, v interface{}) error {
	if err := unmarshal(envs, v); err != nil {
		return err
	}

	if val := getValidator(); val != nil {
		return val.Struct(v)
	}

	return nil
}

func unmarshal(envs map[string]string, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return ErrInvalidValue
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return ErrInvalidValue
	}

	var err error

	t := rv.Type()
	for i := range rv.NumField() {
		valueField := rv.Field(i)
		if valueField.Kind() == reflect.Struct {
			if !valueField.Addr().CanInterface() {
				continue
			}

			if unErr := unmarshal(envs, valueField.Addr().Interface()); unErr != nil {
				err = errors.Join(err, unErr)
				continue
			}
		}

		typeField := t.Field(i)
		tag := typeField.Tag.Get(Tag)
		if tag == "" {
			continue
		}

		if !valueField.CanSet() {
			err = errors.Join(err, fmt.Errorf("field %s is not exported", typeField.Name))
			continue
		}

		tf := parseTag(tag)

		envValue, ok := envs[tf.Key]
		if !ok {
			if tf.Required && tf.Default == "" {
				err = errors.Join(err, fmt.Errorf("required field: %s not found", tf.Key))
				continue
			}

			if tf.Default != "" {
				envValue = tf.Default
			} else {
				continue
			}
		}

		if setErr := set(typeField.Type, valueField, envValue, tf.Separator); setErr != nil {
			err = errors.Join(err, setErr)
			continue
		}

		delete(envs, tf.Key)
	}

	return err
}

func parseTag(tag string) tagField {
	const escapedComma = "\x00"
	tag = strings.ReplaceAll(tag, `\,`, escapedComma)

	envKeys := strings.Split(tag, ",")
	tf := tagField{
		Key:       envKeys[0],
		Separator: Separator,
	}

	for _, key := range envKeys[1:] {
		keyData := strings.SplitN(key, "=", 2)
		switch strings.ToLower(keyData[0]) {
		case "required":
			tf.Required = true
			continue
		case "default":
			if len(keyData) != 2 {
				continue
			}
			tf.Default = strings.ReplaceAll(keyData[1], escapedComma, ",")
			continue
		case "separator":
			if len(keyData) != 2 {
				continue
			}
			tf.Separator = strings.ReplaceAll(keyData[1], escapedComma, ",")
		default:
			continue
		}
	}

	return tf
}

func set(t reflect.Type, f reflect.Value, value, sliceSeparator string) error {
	switch t.Kind() {
	case reflect.Ptr:
		ptr := reflect.New(t.Elem())
		if err := set(t.Elem(), ptr.Elem(), value, sliceSeparator); err != nil {
			return err
		}
		f.Set(ptr)
	case reflect.String:
		f.SetString(value)
	case reflect.Bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		f.SetBool(v)
	case reflect.Float32:
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return err
		}
		f.SetFloat(v)
	case reflect.Float64:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		f.SetFloat(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t.PkgPath() == "time" && t.Name() == "Duration" {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}

			f.Set(reflect.ValueOf(duration))
			break
		}

		v, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		f.SetInt(int64(v))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		f.SetUint(v)
	case reflect.Slice:
		if sliceSeparator == "" {
			sliceSeparator = Separator
		}
		values := strings.Split(value, sliceSeparator)
		switch t.Elem().Kind() {
		case reflect.String:
			f.Set(reflect.ValueOf(values))
		default:
			dest := reflect.MakeSlice(reflect.SliceOf(t.Elem()), len(values), len(values))
			for i, v := range values {
				if err := set(t.Elem(), dest.Index(i), v, sliceSeparator); err != nil {
					return err
				}
			}
			f.Set(dest)
		}
	case reflect.Map:
		if sliceSeparator == "" {
			sliceSeparator = Separator
		}
		if t.Key().Kind() != reflect.String {
			return ErrUnsupportedType
		}
		dest := reflect.MakeMap(t)
		if value == "" {
			f.Set(dest)
			return nil
		}
		pairs := strings.Split(value, sliceSeparator)
		for _, pair := range pairs {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				return fmt.Errorf("invalid map entry: %s", pair)
			}
			keyVal := reflect.New(t.Key()).Elem()
			keyVal.SetString(kv[0])
			valVal := reflect.New(t.Elem()).Elem()
			if err := set(t.Elem(), valVal, kv[1], sliceSeparator); err != nil {
				return err
			}
			dest.SetMapIndex(keyVal, valVal)
		}
		f.Set(dest)
	default:
		return ErrUnsupportedType
	}

	return nil
}
