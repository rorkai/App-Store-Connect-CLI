package asc

import (
	"fmt"
	"reflect"
)

// rowsFunc extracts headers and rows from a typed response value.
type rowsFunc func(data any) ([]string, [][]string, error)

// directRenderFunc renders the value using the provided render callback.
// Used for multi-table types that need to call render more than once.
type directRenderFunc func(data any, render func([]string, [][]string)) error

// outputRegistry maps concrete pointer types to their rows-extraction function.
var outputRegistry = map[reflect.Type]rowsFunc{}

// directRenderRegistry maps types that need direct render control (multi-table output).
var directRenderRegistry = map[reflect.Type]directRenderFunc{}

// registerRows registers a rows function for the given pointer type.
// The function must accept a pointer and return (headers, rows).
func registerRows[T any](fn func(*T) ([]string, [][]string)) {
	t := reflect.TypeFor[*T]()
	if _, exists := outputRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	if _, exists := directRenderRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	outputRegistry[t] = func(data any) ([]string, [][]string, error) {
		h, r := fn(data.(*T))
		return h, r, nil
	}
}

// registerRowsErr registers a rows function that can return an error.
func registerRowsErr[T any](fn func(*T) ([]string, [][]string, error)) {
	t := reflect.TypeFor[*T]()
	if _, exists := outputRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	if _, exists := directRenderRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	outputRegistry[t] = func(data any) ([]string, [][]string, error) {
		return fn(data.(*T))
	}
}

// registerRowsAdapter registers rows rendering by adapting one pointer type to another.
func registerRowsAdapter[T any, U any](adapter func(*T) *U, rows func(*U) ([]string, [][]string)) {
	registerRows(func(v *T) ([]string, [][]string) {
		return rows(adapter(v))
	})
}

// registerSingleResourceRowsAdapter registers rows rendering for list renderers
// by adapting SingleResponse[T] into Response[T] with one item in Data.
func registerSingleResourceRowsAdapter[T any](rows func(*Response[T]) ([]string, [][]string)) {
	registerRowsAdapter(func(v *SingleResponse[T]) *Response[T] {
		return &Response[T]{Data: []Resource[T]{v.Data}}
	}, rows)
}

// registerRowsWithSingleResourceAdapter registers both list and single handlers
// for row renderers that operate on Response[T].
func registerRowsWithSingleResourceAdapter[T any](rows func(*Response[T]) ([]string, [][]string)) {
	registerRows(rows)
	registerSingleResourceRowsAdapter(rows)
}

// registerSingleToListRowsAdapter registers rows rendering by adapting a single
// response struct into a corresponding list response struct using shared field
// names. The source type must expose `Data` and may expose `Links`; the target
// type must expose `Data` as a slice and may expose `Links`.
func registerSingleToListRowsAdapter[T any, U any](rows func(*U) ([]string, [][]string)) {
	registerRows(func(v *T) ([]string, [][]string) {
		source := reflect.ValueOf(v).Elem()
		var target U
		targetValue := reflect.ValueOf(&target).Elem()

		sourceData := source.FieldByName("Data")
		targetData := targetValue.FieldByName("Data")
		if !sourceData.IsValid() || !targetData.IsValid() {
			panic("output registry: single/list adapter requires Data field on source and target")
		}
		if targetData.Kind() != reflect.Slice {
			panic("output registry: single/list adapter target Data field must be a slice")
		}
		targetElemType := targetData.Type().Elem()
		if !sourceData.Type().AssignableTo(targetElemType) {
			panic(fmt.Sprintf(
				"output registry: adapter Data type mismatch source=%s target=%s",
				sourceData.Type(),
				targetElemType,
			))
		}

		rowsSlice := reflect.MakeSlice(targetData.Type(), 1, 1)
		rowsSlice.Index(0).Set(sourceData)
		targetData.Set(rowsSlice)

		sourceLinks := source.FieldByName("Links")
		targetLinks := targetValue.FieldByName("Links")
		if sourceLinks.IsValid() && targetLinks.IsValid() {
			if sourceLinks.Type().AssignableTo(targetLinks.Type()) {
				targetLinks.Set(sourceLinks)
			}
		}

		return rows(&target)
	})
}

// registerRowsWithSingleToListAdapter registers both list and single handlers
// when list rendering expects a concrete list response type.
func registerRowsWithSingleToListAdapter[T any, U any](rows func(*U) ([]string, [][]string)) {
	registerRows(rows)
	registerSingleToListRowsAdapter[T, U](rows)
}

// registerDirect registers a type that needs direct render control (multi-table output).
func registerDirect[T any](fn func(*T, func([]string, [][]string)) error) {
	t := reflect.TypeFor[*T]()
	if _, exists := outputRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	if _, exists := directRenderRegistry[t]; exists {
		panic(fmt.Sprintf("output registry: duplicate registration for %s", t))
	}
	directRenderRegistry[t] = func(data any, render func([]string, [][]string)) error {
		return fn(data.(*T), render)
	}
}

// renderByRegistry looks up the rows function for the given value and renders
// using the provided render function (RenderTable or RenderMarkdown).
// Falls back to JSON output for unregistered types.
func renderByRegistry(data any, render func([]string, [][]string)) error {
	t := reflect.TypeOf(data)

	// Check direct render registry first (multi-table types).
	if fn, ok := directRenderRegistry[t]; ok {
		return fn(data, render)
	}

	// Standard single-table types.
	if fn, ok := outputRegistry[t]; ok {
		h, r, err := fn(data)
		if err != nil {
			return err
		}
		render(h, r)
		return nil
	}

	return PrintJSON(data)
}
