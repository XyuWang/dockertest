package dockertest

import "fmt"

var _hooks = make(map[string]HookFunc)

type HookFunc func(*Container) error

func Register(name string, hookFunc HookFunc) {
	if _hooks[name] != nil {
		panic(fmt.Sprintf("%s already registed", name))
	}
	_hooks[name] = hookFunc
}
func init() {
	Register("refresh_mysql", mysqlHook)
}
