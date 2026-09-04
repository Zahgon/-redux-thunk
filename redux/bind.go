package redux

import (
	"fmt"
	"reflect"
)

var anyType = reflect.TypeOf((*any)(nil)).Elem()

// BindActionCreator is the Go analogue of redux's internal `bindActionCreator`:
//
//	(...args) => dispatch(actionCreator(...args))
//
// The returned value is a function with the *same parameter list* as the
// supplied action creator, but returning `any` -- the value returned by
// dispatch. This is deliberate and mirrors an important quirk of the original:
// for a plain action creator dispatch returns the action, but for a thunk
// action creator dispatch returns the thunk's return value. TypeScript models
// this with the `ThunkActionDispatch` conditional type; Go cannot express the
// transformation generically, so the runtime helper erases the result and the
// thunk package supplies statically-typed Bind0..Bind3 alternatives.
//
// Panics if actionCreator is not a function returning exactly one value.
func BindActionCreator(actionCreator any, dispatch Dispatch) any {
	v := reflect.ValueOf(actionCreator)
	if !v.IsValid() || v.Kind() != reflect.Func {
		panic(&Error{Message: fmt.Sprintf(
			"bindActionCreators expected a function actionCreator, instead received %T", actionCreator)})
	}
	t := v.Type()
	if t.NumOut() != 1 {
		panic(&Error{Message: fmt.Sprintf(
			"bindActionCreators expected an action creator returning exactly one value, got %d", t.NumOut())})
	}

	in := make([]reflect.Type, t.NumIn())
	for i := range in {
		in[i] = t.In(i)
	}
	boundType := reflect.FuncOf(in, []reflect.Type{anyType}, t.IsVariadic())

	bound := reflect.MakeFunc(boundType, func(args []reflect.Value) []reflect.Value {
		var results []reflect.Value
		if t.IsVariadic() {
			results = v.CallSlice(args)
		} else {
			results = v.Call(args)
		}
		if dispatch == nil {
			panic(&Error{Message: "dispatch is not a function"})
		}
		out := dispatch(results[0].Interface())
		res := reflect.New(anyType).Elem()
		if out != nil {
			res.Set(reflect.ValueOf(out))
		}
		return []reflect.Value{res}
	})

	return bound.Interface()
}

// BindActionCreators is the Go analogue of redux's `bindActionCreators` for the
// object form:
//
//	const actions = bindActionCreators({ a, b, c }, store.dispatch)
//
// Entries whose value is not a function are skipped rather than rejected, which
// is what Redux does -- its loop binds a key only `if (typeof actionCreator ===
// 'function')`. The result map has the surviving keys, each holding a function
// with the original parameter list returning `any`.
//
// A nil dispatch is accepted here and rejected when a bound creator is called,
// matching Redux: binding never inspects dispatch, so `dispatch is not a
// function` surfaces at call time.
func BindActionCreators(actionCreators map[string]any, dispatch Dispatch) map[string]any {
	if actionCreators == nil {
		panic(&Error{Message: "bindActionCreators expected an object or a function, but instead received: 'null'. " +
			`Did you write "import ActionCreators from" instead of "import * as ActionCreators from"?`})
	}
	bound := make(map[string]any, len(actionCreators))
	for key, creator := range actionCreators {
		v := reflect.ValueOf(creator)
		if !v.IsValid() || v.Kind() != reflect.Func {
			continue
		}
		bound[key] = BindActionCreator(creator, dispatch)
	}
	return bound
}
