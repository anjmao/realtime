# Suspend-aware monotonic timers in pure Go

This "realtime" package is proof of concept for [real time timers proposal](https://github.com/golang/go/issues/36141). Package implements part of Go standard library time package functionality using platform specific APIS.

✅ - Done.

❎ - Works, but could be missing something or need improvements.

❌ - Not started yet.

| API           | Darwin/BSD    |  Linux        | Windows
| ------------- | ------------- | ------------- |------------- |
| `realtime.Now()`                                | ✅️ | ✅️ | ❌ 
| `realtime.Since(u time.Duration)`               | ✅ | ✅️ | ❌ 
| `realtime.Sleep(d time.Duration)`               | ❎️ | ❎️ | ❌ 
| `realtime.AfterFunc(d time.Duration, f func())` | ❎️ | ❎ | ❌ 
| `realtime.After(d time.Duration)`               | ❎️ | ❎️ | ❌
| `realtime.NewTimer(d time.Duration)`            | ❎️ | ❎️ | ❌
| `realtime.Tick(d time.Duration)`                | ❎️ | ❎️ | ❌
| `realtime.NewTicker(d time.Duration)`           | ❎️ | ❎️ | ❌

