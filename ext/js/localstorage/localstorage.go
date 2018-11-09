package localstorage

import "github.com/gopherjs/gopherjs/js"

func Get(key string) string {
	res := js.Global.Get("localStorage").Call("getItem", key)
	if res.Bool() {
		return res.String()
	}
	return ""
}

func Set(key, value string) {
	js.Global.Get("localStorage").Call("setItem", key, value)
}
