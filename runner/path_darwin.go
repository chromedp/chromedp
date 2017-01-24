// +build darwin

package runner

const (
	DefaultChromePath = `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
)

func findChromePath() string {
	return DefaultChromePath
}
