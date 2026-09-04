package redux

import (
	"fmt"
	"reflect"
)

// Redux validates every dispatched action before running the reducer. The two
// guards below are the ones a thunk user hits most often -- dispatching a
// function without installing the middleware trips the first one, and it is the
// error message that tells them why. Reproducing them is therefore part of
// being a faithful port, not a nicety.
const (
	plainActionAdvice = "You may need to add middleware to your store setup to handle dispatching other values, " +
		"such as 'redux-thunk' to handle dispatching functions. " +
		"See https://redux.js.org/tutorials/fundamentals/part-4-store#middleware and " +
		"https://redux.js.org/tutorials/fundamentals/part-6-async-logic#using-the-redux-thunk-middleware for examples."

	undefinedTypeAdvice = `Actions may not have an undefined "type" property. ` +
		"You may have misspelled an action type string constant."
)

// The JavaScript `typeof` vocabulary redux quotes back at the caller: these are
// deliberately not the Go kind names ("func", "slice", "int").
const (
	kindFunction  = "function"
	kindString    = "string"
	kindBoolean   = "boolean"
	kindArray     = "array"
	kindNumber    = "number"
	kindUndefined = "undefined"
	kindNull      = "null"
)

// typeKeyValue is hoisted because reflect.ValueOf boxes the string, which would
// otherwise allocate on every single dispatch.
var typeKeyValue = reflect.ValueOf(TypeKey)

func errNotPlainAction(kind string) *Error {
	return &Error{Message: fmt.Sprintf(
		"Actions must be plain objects. Instead, the actual type was: '%s'. %s", kind, plainActionAdvice)}
}

func errUndefinedActionType() *Error {
	return &Error{Message: undefinedTypeAdvice}
}

func errNonStringActionType(actionType any) *Error {
	return &Error{Message: fmt.Sprintf(
		`Action "type" property must be a string. Instead, the actual type was: '%s'. Value was: '%v' (stringified)`,
		kindOf(reflect.TypeOf(actionType)), actionType)}
}

// assertPlainAction is the Go analogue of redux's `isPlainObject(action)` and
// `typeof action.type === 'undefined'` checks.
//
// "Plain object" maps to: a value implementing Action, or a string-keyed map, or
// a struct. Everything else -- functions above all -- is rejected the way
// JavaScript rejects a non-plain action.
func assertPlainAction(action any) {
	// Every action this port can construct is one of these three shapes, and
	// reflect.Value.MapIndex allocates a box for the element it returns, so the
	// common path deliberately avoids reflection altogether. The map cases must
	// precede the Action case: AnyAction satisfies Action, but its ActionType
	// cannot distinguish an absent `type` from one of the wrong kind.
	switch a := action.(type) {
	case AnyAction:
		assertMapActionType(a)
		return
	case map[string]any:
		assertMapActionType(a)
		return
	case Action:
		return
	}

	v := reflect.ValueOf(action)
	if !v.IsValid() {
		panic(errNotPlainAction(kindUndefined))
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			panic(errNotPlainAction(kindNull))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			panic(errNotPlainAction(kindOf(v.Type())))
		}
		key := typeKeyValue
		if v.Type().Key() != key.Type() {
			key = key.Convert(v.Type().Key())
		}
		entry := v.MapIndex(key)
		if !entry.IsValid() {
			panic(errUndefinedActionType())
		}
		// Redux's third guard. An empty string is a valid type; a non-string is
		// not, and conflating the two would report "undefined" for a `type` that
		// is present but of the wrong kind. The kind is inspected rather than
		// the boxed value so that the happy path does not allocate.
		if entry.Kind() == reflect.Interface {
			entry = entry.Elem()
		}
		if !entry.IsValid() {
			panic(errUndefinedActionType())
		}
		if entry.Kind() != reflect.String {
			panic(errNonStringActionType(entry.Interface()))
		}
	case reflect.Struct:
		// A struct is the closest Go analogue of an object literal, but unlike
		// JavaScript it carries no `type` property unless it implements Action,
		// whose ActionType is a string by construction.
		if _, ok := action.(Action); !ok {
			panic(errUndefinedActionType())
		}
	default:
		panic(errNotPlainAction(kindOf(v.Type())))
	}
}

func assertMapActionType(action map[string]any) {
	actionType, present := action[TypeKey]
	if !present || actionType == nil {
		panic(errUndefinedActionType())
	}
	if _, ok := actionType.(string); !ok {
		panic(errNonStringActionType(actionType))
	}
}

// kindOf mirrors redux's `kindOf` helper, which names the offending value in
// the error message using JavaScript's vocabulary rather than Go's.
func kindOf(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Func:
		return kindFunction
	case reflect.String:
		return kindString
	case reflect.Bool:
		return kindBoolean
	case reflect.Slice, reflect.Array:
		return kindArray
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return kindNumber
	default:
		return t.String()
	}
}
