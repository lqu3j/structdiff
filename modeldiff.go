package modeldiff

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	tag = "comparedby"
)

type DiffDetails struct {
	Change map[string]interface{} // 修改
	Del    map[string]interface{} // 删除
	Add    map[string]interface{} // 增加
}

func Diff(new, old interface{}) (*DiffDetails, error) {
	diff := &DiffDetails{}

	if err := compare("", "", false, diff, new, old); err != nil {
		return nil, err
	}
	return diff, nil
}

func compare(key, comparedby string, isDirect bool, diff *DiffDetails, new, old interface{}) error {
	if comparedby == "-" {
		return nil
	}

	newT := reflect.TypeOf(new)
	oldT := reflect.TypeOf(old)
	if newT.Name() != oldT.Name() {
		return fmt.Errorf("can't compare, not same type, newType:%v, oldType:%v", newT.Name(), oldT.Name())
	}
	tp := reflect.TypeOf(new)

	switch tp.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
		if comparedby != "" {
			return fmt.Errorf("basic type can't assign comparedby, path:%v", key)
		}
		if !reflect.DeepEqual(new, old) {
			if diff.Change == nil {
				diff.Change = make(map[string]interface{})
			}
			diff.Change[key] = old
		}
	case reflect.Array, reflect.Slice:
		newV := reflect.ValueOf(new)
		oldV := reflect.ValueOf(old)

		if comparedby != "" {
			if err := compareBySlice(newV, oldV, comparedby, isDirect, key, diff); err != nil {
				return err
			}
		} else {
			// TODO 不能进行字段忽略操作, 有时间可以优化
			if !reflect.DeepEqual(new, old) {
				if diff.Change == nil {
					diff.Change = make(map[string]interface{})
				}
				diff.Change[key] = oldV.Interface()
			}
		}

	case reflect.Ptr, reflect.Interface:
		if reflect.ValueOf(new).IsNil() && reflect.ValueOf(old).IsNil() {
			return nil
		}
		if reflect.ValueOf(new).IsNil() || reflect.ValueOf(old).IsNil() {
			if diff.Change == nil {
				diff.Change = make(map[string]interface{})
			}
			diff.Change[key] = old
		}
		return compare(key, comparedby, isDirect, diff, reflect.ValueOf(new).Elem().Interface(), reflect.ValueOf(old).Elem().Interface())
	case reflect.Struct:
		newV := reflect.ValueOf(new)
		oldV := reflect.ValueOf(old)

		if isDirect {
			equal, err := deepEqual(new, old)
			if err != nil {
				return err
			}
			if !equal {
				if diff.Change == nil {
					diff.Change = make(map[string]interface{})
				}
				diff.Change[key] = oldV.Interface()
			}
		} else {
			for i := 0; i < tp.NumField(); i++ {
				if !tp.Field(i).IsExported() {
					continue
				}

				comparedby, isDirect := parseTag(tp.Field(i).Tag.Get(tag))
				if err := compare(generate(key, tp.Field(i).Name), comparedby, isDirect, diff, newV.Field(i).Interface(), oldV.Field(i).Interface()); err != nil {
					return err
				}
			}
		}

	case reflect.Map:
		if comparedby != "" {
			return fmt.Errorf("basic type can't assign comparedby, path:%v", key)
		}

		newV := reflect.ValueOf(new)
		oldV := reflect.ValueOf(old)

		newIter := newV.MapRange()
		oldIter := oldV.MapRange()

		for newIter.Next() {
			name, err := toString(newIter.Key().Interface())
			if err != nil {
				return err
			}
			if oldV.MapIndex(newIter.Key()).IsValid() {
				if err := compare(generate(key, name), comparedby, isDirect, diff, newIter.Value().Interface(), oldV.MapIndex(newIter.Key()).Interface()); err != nil {
					return err
				}
			} else {
				if diff.Add == nil {
					diff.Add = make(map[string]interface{})
				}
				diff.Add[generate(key, name)] = nil
			}
		}

		for oldIter.Next() {
			name, err := toString(oldIter.Key().Interface())
			if err != nil {
				return err
			}

			if !newV.MapIndex(oldIter.Key()).IsValid() {
				if diff.Del == nil {
					diff.Del = make(map[string]interface{})
				}
				diff.Del[generate(key, name)] = oldIter.Value().Interface()
			}
		}

	}
	return nil
}

func compareBySlice(new, old reflect.Value, comparedby string, isDirect bool, key string, diff *DiffDetails) error {
	if new.Kind() != reflect.Array && new.Kind() != reflect.Slice {
		return fmt.Errorf("can't execute compareBySlice, not valid type, type=%v", new.Type().Name())
	}

	newV := new
	oldV := old

	for i := 0; i < newV.Len(); i++ {
		value, err := getElement(newV.Index(i))
		if err != nil {
			return err
		}
		oldElement, err := getFromSlice(oldV, value, comparedby)
		if err != nil {
			return err
		}

		field, _ := value.Type().FieldByName(comparedby)
		if !field.IsExported() {
			return fmt.Errorf("can't compare, is not export field")
		}
		comparedvalue, err := getComparedValue(value.FieldByName(comparedby))
		if err != nil {
			return err
		}
		if !oldElement.IsZero() {
			if isDirect {
				equal, err := deepEqual(value.Interface(), oldElement.Interface())
				if err != nil {
					return err
				}
				if !equal {
					if diff.Change == nil {
						diff.Change = make(map[string]interface{})
					}
					diff.Change[fmt.Sprintf(`%v.#(%v==%v)`, key, comparedby, comparedvalue)] = oldElement.Interface()
				}
			} else {
				tmp := &DiffDetails{}
				if err := compare("", "", isDirect, tmp, value.Interface(), oldElement.Interface()); err != nil {
					return err
				}
				for k, v := range tmp.Add {
					if diff.Add == nil {
						diff.Add = make(map[string]interface{})
					}
					diff.Add[fmt.Sprintf(`%v.#(%v==%v).%v`, key, comparedby, comparedvalue, k)] = v
				}
				for k, v := range tmp.Change {
					if diff.Change == nil {
						diff.Change = make(map[string]interface{})
					}
					diff.Change[fmt.Sprintf(`%v.#(%v==%v).%v`, key, comparedby, comparedvalue, k)] = v
				}
				for k, v := range tmp.Del {
					if diff.Del == nil {
						diff.Del = make(map[string]interface{})
					}
					diff.Del[fmt.Sprintf(`%v.#(%v==%v).%v`, key, comparedby, comparedvalue, k)] = v
				}
			}
		} else {
			if diff.Add == nil {
				diff.Add = make(map[string]interface{})
			}
			diff.Add[fmt.Sprintf(`%v.#(%v==%v)`, key, comparedby, comparedvalue)] = nil
		}
	}

	for i := 0; i < oldV.Len(); i++ {
		value, err := getElement(oldV.Index(i))
		if err != nil {
			return err
		}
		newElement, err := getFromSlice(newV, value, comparedby)
		if err != nil {
			return err
		}

		comparedvalue, err := getComparedValue(value.FieldByName(comparedby))
		if err != nil {
			return err
		}
		if newElement.IsZero() {
			if diff.Del == nil {
				diff.Del = make(map[string]interface{})
			}
			diff.Del[fmt.Sprintf(`%v.#(%v==%v)`, key, comparedby, comparedvalue)] = value.Interface()
		}
	}
	return nil
}

func getFromSlice(slice reflect.Value, value reflect.Value, comparedby string) (reflect.Value, error) {
	for i := 0; i < slice.Len(); i++ {
		src, err := getElement(slice.Index(i))
		if err != nil {
			return reflect.Zero(src.Type()), err
		}

		srcFiled := src.FieldByName(comparedby)
		if srcFiled.IsZero() {
			return reflect.Zero(src.Type()), fmt.Errorf("can't find this comparedby field, comparedby=%v", comparedby)
		}

		dstFiled := value.FieldByName(comparedby)
		if dstFiled.IsZero() {
			return reflect.Zero(src.Type()), fmt.Errorf("can't find this comparedby field, comparedby=%v", comparedby)
		}

		if reflect.DeepEqual(srcFiled.Interface(), dstFiled.Interface()) {
			return src, nil
		}
	}
	return reflect.Zero(value.Type()), nil
}

func getElement(value reflect.Value) (reflect.Value, error) {
	switch value.Kind() {
	case reflect.Interface, reflect.Ptr:
		return getElement(value.Elem())
	case reflect.Struct:
		return value, nil
	default:
		return reflect.Zero(value.Type()), fmt.Errorf("basic type can't assign comparedby")
	}
}

func getComparedValue(value reflect.Value) (string, error) {
	switch value.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", value.Interface()), nil
	case reflect.String:
		return fmt.Sprintf(`"%v"`, value), nil
	case reflect.Interface, reflect.Ptr:
		return getComparedValue(value.Elem())
	default:
		return "", fmt.Errorf("invalid type")
	}
}

func generate(key, name string) string {
	if key == "" {
		return name
	}
	return key + "." + name
}

func toString(value interface{}) (string, error) {
	tp := reflect.TypeOf(value)

	switch tp.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.String, reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", value), nil
	case reflect.Ptr, reflect.Interface:
		return toString(reflect.ValueOf(value).Interface())
	}
	return "", fmt.Errorf("not support key")
}

func deepEqual(new, old interface{}) (bool, error) {
	diff, err := Diff(new, old)
	if err != nil {
		return false, err
	}

	if len(diff.Change) == 0 && len(diff.Add) == 0 && len(diff.Del) == 0 {
		return true, nil
	} else {
		return false, nil
	}
}

// struct: 可以设置是否直接比较结构体
// array or slice:  可以设置与哪个对象比较
func parseTag(value string) (string, bool) {
	isDirect := false
	comparedby := value
	array := strings.Split(value, ",")
	if len(array) == 2 {
		comparedby = array[0]
		if array[1] == "direct" {
			isDirect = true
		}
	}
	return comparedby, isDirect
}
