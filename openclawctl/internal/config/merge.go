package config

import "reflect"

func MergeMaps(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	if overlay == nil {
		return base
	}

	for key, value := range overlay {
		if existing, ok := base[key]; ok {
			existingMap, existingOk := toMap(existing)
			valueMap, valueOk := toMap(value)
			if existingOk && valueOk {
				base[key] = MergeMaps(existingMap, valueMap)
				continue
			}
		}
		base[key] = value
	}
	return base
}

func SetPath(root map[string]any, path []string, value any) {
	current := root
	for i, segment := range path {
		if i == len(path)-1 {
			current[segment] = value
			return
		}
		next, ok := current[segment]
		if !ok {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}
		nextMap, ok := toMap(next)
		if !ok {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}
		current = nextMap
	}
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		out := map[string]any{}
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = iter.Value().Interface()
		}
		return out, true
	}
	return nil, false
}
