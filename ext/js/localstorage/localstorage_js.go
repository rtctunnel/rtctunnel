//+build js

package localstorage

import "syscall/js"

func Get(key string) string {
	res := js.Global().Get("localStorage").Call("getItem", key)
	if res.Truthy() {
		return res.String()
	}
	return ""
}

func Set(key, value string) {
	js.Global().Get("localStorage").Call("setItem", key, value)
}
