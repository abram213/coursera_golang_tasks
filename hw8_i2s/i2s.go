package main

import (
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	outType := reflect.TypeOf(out).Kind()
	if outType != reflect.Ptr {
		return fmt.Errorf("%v not pointer value", out)
	}
	outElemType := reflect.TypeOf(out).Elem().Kind()
	switch outElemType {
	case reflect.Struct:
		if err := unmarshalStruct(data, out); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if err := unmarshalSlice(data, out); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported out value type %v", outElemType)
	}
	return nil
}

func unmarshalStruct(data interface{}, out interface{}) error {
	dataType := reflect.TypeOf(data).Kind()
	if dataType != reflect.Map {
		return fmt.Errorf("invalid data type got: %v, want: %v", dataType, reflect.Map)
	}

	dataValue := reflect.ValueOf(data)
	outElem := reflect.ValueOf(out).Elem()
	for i := 0; i < outElem.NumField(); i++ {
		outFieldName := outElem.Type().Field(i).Name
		outField := outElem.Field(i)
		key, ok := keyIndex(dataValue.MapKeys(), outFieldName)
		if !ok {
			continue
		}
		dataKeyValue := dataValue.MapIndex(key)
		dKVType := dataKeyValue.Elem().Kind()
		switch outField.Kind() {
		case reflect.Int:
			if dKVType != reflect.Float64 {
				return fmt.Errorf("invalid value type got: %v, want: %v", dKVType, reflect.Float64)
			}
			fValue := dataKeyValue.Elem().Float()
			outField.SetInt(int64(fValue))
		case reflect.String:
			if dKVType != reflect.String {
				return fmt.Errorf("invalid value type got: %v, want: %v", dKVType, reflect.String)
			}
			outField.SetString(dataKeyValue.Elem().String())
		case reflect.Bool:
			if dKVType != reflect.Bool {
				return fmt.Errorf("invalid value type got: %v, want: %v", dKVType, reflect.Bool)
			}
			outField.SetBool(dataKeyValue.Elem().Bool())
		case reflect.Array, reflect.Slice:
			if dKVType != reflect.Slice {
				return fmt.Errorf("invalid value type got: %v, want: %v", dKVType, reflect.Slice)
			}
			if err := i2s(dataValue.MapIndex(key).Interface(), outField.Addr().Interface()); err != nil {
				return err
			}
		case reflect.Struct:
			if dKVType != reflect.Map {
				return fmt.Errorf("invalid value type got: %v, want: %v", dKVType, reflect.Map)
			}
			if err := i2s(dataKeyValue.Interface(), outField.Addr().Interface()); err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalSlice(data interface{}, out interface{}) error {
	dataType := reflect.TypeOf(data).Kind()
	if dataType != reflect.Slice {
		return fmt.Errorf("bad data structure got: %v, want: %v", dataType, reflect.Slice)
	}
	outElem := reflect.ValueOf(out).Elem()

	dataValue := reflect.ValueOf(data)
	for i := 0; i < dataValue.Len(); i++ {
		elemType := reflect.TypeOf(out).Elem().Elem()
		outElem.Set(reflect.Append(outElem, reflect.Zero(elemType)))
		if err := i2s(dataValue.Index(i).Interface(), outElem.Index(i).Addr().Interface()); err != nil {
			return err
		}
	}
	return nil
}

func keyIndex(keys []reflect.Value, key string) (reflect.Value, bool) {
	for _, v := range keys {
		if key == v.String() {
			return v, true
		}
	}
	return reflect.Value{}, false
}
