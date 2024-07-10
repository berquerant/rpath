package rpath

var (
	enableDebug = false
)

func EnableDebug() {
	enableDebug = true
}

func OnDebug(f func()) {
	if enableDebug {
		f()
	}
}
