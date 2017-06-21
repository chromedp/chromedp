package runner

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

// getMajorVersion returns the major component of the version number of the
// specified program's --version output. E.g: "Google Chrome 59.foo.bar" => 59.
func getMajorVersion(path string) (int, error) {
	version, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return 0, err
	}
	ret := regexp.MustCompile("[^0-9]+([0-9]+)").FindSubmatch(version)
	if ret == nil || len(ret) < 2 {
		fmtStr := "no version number found in version string %s"
		return 0, fmt.Errorf(fmtStr, version)
	}
	majorVersion, err := strconv.Atoi(string(ret[1]))
	if err != nil {
		return 0, err
	}
	return majorVersion, nil
}

// checkHeadlessSupport returns true of the given Chrome binary is thought to
// support the --headless command line option.
func checkHeadlessSupport(path string) bool {
	version, err := getMajorVersion(path)
	if err != nil {
		return false // unknown, assume unsupported.
	}

	return version >= 59 // Headless support arrived in 59.
}
