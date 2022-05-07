//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	out := flag.String("out", "kb.go", "out source")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(out string) error {
	// special characters
	keys := map[rune]Key{
		'\b': {"Backspace", "Backspace", "", "", int64('\b'), int64('\b'), false, false},
		'\t': {"Tab", "Tab", "", "", int64('\t'), int64('\t'), false, false},
		'\r': {"Enter", "Enter", "\r", "\r", int64('\r'), int64('\r'), false, true},
	}
	// load keys
	if err := loadKeys(keys); err != nil {
		return err
	}
	// process keys
	constBuf, mapBuf, err := processKeys(keys)
	if err != nil {
		return err
	}
	src, err := format.Source([]byte(fmt.Sprintf(hdr, string(constBuf), string(mapBuf))))
	if err != nil {
		return err
	}
	return ioutil.WriteFile(out, src, 0o644)
}

// loadKeys loads the dom key definitions from the chromium source tree.
func loadKeys(keys map[rune]Key) error {
	// load key converter data
	domCodeMap, err := loadDomCodeData()
	if err != nil {
		return err
	}
	if len(domCodeMap) == 0 {
		return errors.New("no dom codes defined")
	}
	// load dom code map
	domKeyMap, err := loadDomKeyData()
	if err != nil {
		return err
	}
	if len(domKeyMap) == 0 {
		return errors.New("no dom keys defined")
	}
	// load US layout data
	layoutBuf, err := grab(domUsLayoutDataH)
	if err != nil {
		return err
	}
	// load scan code map
	scanCodeMap, err := loadScanCodes(domCodeMap, domKeyMap, layoutBuf)
	if err != nil {
		return err
	}
	// process printable
	err = loadPrintable(keys, domCodeMap, domKeyMap, layoutBuf, scanCodeMap)
	if err != nil {
		return err
	}
	// process non-printable
	err = loadNonPrintable(keys, domCodeMap, domKeyMap, layoutBuf, scanCodeMap)
	if err != nil {
		return err
	}
	return nil
}

var (
	fixRE    = regexp.MustCompile(`,\n\s{10,}`)
	usbKeyRE = regexp.MustCompile(`(?m)^\s*DOM_CODE\((.*?), (.*?), (.*?), (.*?), (.*?), (.*?), (.*?)\)`)
)

// loadDomCodeData loads the key codes from the dom_code_data.inc.
func loadDomCodeData() (map[string][]string, error) {
	buf, err := grab(domCodeDataInc)
	if err != nil {
		return nil, err
	}
	buf = fixRE.ReplaceAllLiteral(buf, []byte(", "))
	domMap := make(map[string][]string)
	matches := usbKeyRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		vkey := m[7]
		if _, ok := domMap[vkey]; ok {
			panic(fmt.Sprintf("vkey %s already defined", vkey))
		}
		domMap[vkey] = m[1:]
	}
	return domMap, nil
}

// decodeRune is a wrapper around parsing a printable c++ int/char definition to a unicode
// rune value.
func decodeRune(s string) rune {
	if strings.HasPrefix(s, "0x") {
		i, err := strconv.ParseInt(s, 0, 16)
		if err != nil {
			panic(err)
		}
		return rune(i)
	}
	if !strings.HasPrefix(s, "'") || !strings.HasSuffix(s, "'") {
		panic(fmt.Sprintf("expected character, got: %s", s))
	}
	if len(s) == 4 {
		if s[1] != '\\' {
			panic(fmt.Sprintf("expected escaped character, got: %s", s))
		}
		return rune(s[2])
	}
	if len(s) != 3 {
		panic(fmt.Sprintf("expected character, got: %s", s))
	}
	return rune(s[1])
}

// getCode is a simple wrapper around parsing the code definition.
func getCode(s string) string {
	if !strings.HasPrefix(s, `"`) || !strings.HasSuffix(s, `"`) {
		panic(fmt.Sprintf("expected string, got: %s", s))
	}
	return s[1 : len(s)-1]
}

// addKey is a simple map add wrapper to panic if the key is already defined,
// and to lookup the correct scan code.
func addKey(keys map[rune]Key, r rune, key Key, scanCodeMap map[string][]int64, shouldPanic bool) {
	if _, ok := keys[r]; ok {
		if shouldPanic {
			panic(fmt.Sprintf("rune %U (%s/%s) already defined in keys", r, key.Code, key.Key))
		}
		return
	}
	sc, ok := scanCodeMap[key.Code]
	if ok {
		key.Native = sc[0]
		key.Windows = sc[1]
	}
	keys[r] = key
}

var printableKeyRE = regexp.MustCompile(`\{DomCode::(.+?), \{(.+?), (.+?)\}\}`)

// loadPrintable loads the printable key definitions.
func loadPrintable(keys map[rune]Key, domCodeMap, domKeyMap map[string][]string, layoutBuf []byte, scanCodeMap map[string][]int64) error {
	buf := extract(layoutBuf, "kPrintableCodeMap")
	matches := printableKeyRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		domCode := m[1]
		// ignore domCodes that are duplicates of other unicode characters
		if domCode == "INTL_BACKSLASH" || domCode == "INTL_HASH" || strings.HasPrefix(domCode, "NUMPAD") {
			continue
		}
		kc, ok := domCodeMap[domCode]
		if !ok {
			panic(fmt.Sprintf("could not find key %s in dom code map", domCode))
		}
		if kc[5] == "NULL" {
			continue
		}
		code := getCode(kc[5])
		r1, r2 := decodeRune(m[2]), decodeRune(m[3])
		addKey(keys, r1, Key{
			Code:       code,
			Key:        string(r1),
			Text:       string(r1),
			Unmodified: string(r1),
			Print:      true,
		}, scanCodeMap, true)
		// shifted value is same as non-shifted, so skip
		if r2 == r1 {
			continue
		}
		// skip for duplicate keys
		if r2 == '|' && domCode != "BACKSLASH" {
			continue
		}
		addKey(keys, r2, Key{
			Code:       code,
			Key:        string(r2),
			Text:       string(r2),
			Unmodified: string(r1),
			Shift:      true,
			Print:      true,
		}, scanCodeMap, true)
	}
	return nil
}

var domKeyRE = regexp.MustCompile(`(?m)^\s+DOM_KEY_(?:UNI|MAP)\("(.+?)",\s*(.+?),\s*(0x[0-9A-F]{4})\)`)

// loadDomKeyData loads the dom key data definitions.
func loadDomKeyData() (map[string][]string, error) {
	buf, err := grab(domKeyDataInc)
	if err != nil {
		return nil, err
	}
	buf = fixRE.ReplaceAllLiteral(buf, []byte(", "))
	keyMap := make(map[string][]string)
	matches := domKeyRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		keyMap[m[2]] = m[1:]
	}
	return keyMap, nil
}

var nonPrintableKeyRE = regexp.MustCompile(`\n\s{4}\{DomCode::(.+?), DomKey::(.+?)\}`)

// loadNonPrintable loads the not printable key definitions.
func loadNonPrintable(keys map[rune]Key, domCodeMap, domKeyMap map[string][]string, layoutBuf []byte, scanCodeMap map[string][]int64) error {
	buf := extract(layoutBuf, "kNonPrintableCodeMap")
	matches := nonPrintableKeyRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		code, key := m[1], m[2]
		// get code, key definitions
		dc, ok := domCodeMap[code]
		if !ok {
			panic(fmt.Sprintf("no dom code definition for %s", code))
		}
		dk, ok := domKeyMap[key]
		if !ok {
			panic(fmt.Sprintf("no dom key definition for %s", key))
		}
		// some scan codes do not have names defined, so use key name
		c := dk[0]
		if dc[5] != "NULL" {
			c = getCode(dc[5])
		}
		// convert rune
		r, err := strconv.ParseInt(dk[2], 0, 32)
		if err != nil {
			return err
		}
		addKey(keys, rune(r), Key{
			Code: c,
			Key:  dk[0],
		}, scanCodeMap, false)
	}
	return nil
}

var nameRE = regexp.MustCompile(`[A-Z][a-z]+:`)

// processKeys processes the generated keys.
func processKeys(keys map[rune]Key) ([]byte, []byte, error) {
	// order rune keys
	idx := make([]rune, len(keys))
	var i int
	for c := range keys {
		idx[i] = c
		i++
	}
	sort.Slice(idx, func(a, b int) bool {
		return idx[a] < idx[b]
	})
	// process
	var constBuf, mapBuf bytes.Buffer
	for _, c := range idx {
		key := keys[c]
		g, isGoCode := goCodes[c]
		s := fmt.Sprintf("\\u%04x", c)
		if isGoCode {
			s = g
		} else if key.Print {
			s = fmt.Sprintf("%c", c)
		}
		// add key definition
		v := nameRE.ReplaceAllString(strings.TrimPrefix(fmt.Sprintf("%#v", key), "main."), "")
		mapBuf.WriteString(fmt.Sprintf("'%s': &%s,\n", s, v))
		// fix 'Quote' const
		if s == `\'` {
			s = `'`
		}
		// add const definition
		if (isGoCode && c != '\n') || !key.Print {
			n := strings.TrimPrefix(key.Key, ".")
			if n == `'` || n == `\` {
				n = key.Code
			}
			constBuf.WriteString(fmt.Sprintf("%s = \"%s\"\n", n, s))
		}
	}
	return constBuf.Bytes(), mapBuf.Bytes(), nil
}

var (
	domCodeVkeyFixRE = regexp.MustCompile(`,\n\s{5,}`)
	domCodeVkeyRE    = regexp.MustCompile(`(?m)^\s*\{DomCode::(.+?), (.+?)\}`)
)

// loadScanCodes loads the scan codes for the dom key definitions.
func loadScanCodes(domCodeMap, domKeyMap map[string][]string, layoutBuf []byte) (map[string][]int64, error) {
	vkeyCodeMap, err := loadPosixWinKeyboardCodes()
	if err != nil {
		return nil, err
	}
	buf := extract(layoutBuf, "kDomCodeToKeyboardCodeMap")
	buf = domCodeVkeyFixRE.ReplaceAllLiteral(buf, []byte(", "))
	scanCodeMap := make(map[string][]int64)
	matches := domCodeVkeyRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		domCode, vkey := m[1], m[2]
		kc, ok := domCodeMap[domCode]
		if !ok {
			panic(fmt.Sprintf("dom code %s not defined in dom code map", domCode))
		}
		sc, ok := vkeyCodeMap[vkey]
		if !ok {
			panic(fmt.Sprintf("vkey %s is not defined in vkey code map", vkey))
		}
		if kc[5] == "NULL" {
			continue
		}
		scanCodeMap[getCode(kc[5])] = sc
	}
	return scanCodeMap, nil
}

var defineRE = regexp.MustCompile(`(?m)^#define\s+(.+?)\s+([0-9A-Fx]+)`)

// loadPosixWinKeyboardCodes loads the native and windows keyboard scan codes
// mapped to the DOM key.
func loadPosixWinKeyboardCodes() (map[string][]int64, error) {
	lookup := map[string]string{
		// mac alias
		"VKEY_LWIN": "0x5B",
		// no idea where these are defined in chromium code base (assuming in
		// windows headers)
		//
		// manually added here as pulled from various online docs
		"VK_CANCEL":       "0x03",
		"VK_OEM_ATTN":     "0xF0",
		"VK_OEM_FINISH":   "0xF1",
		"VK_OEM_COPY":     "0xF2",
		"VK_DBE_SBCSCHAR": "0xF3",
		"VK_DBE_DBCSCHAR": "0xF4",
		"VK_OEM_BACKTAB":  "0xF5",
		"VK_OEM_AX":       "0xE1",
	}
	// load windows key lookups
	buf, err := grab(windowsKeyboardCodesH)
	if err != nil {
		return nil, err
	}
	matches := defineRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		lookup[m[1]] = m[2]
	}
	// load posix and win keyboard codes
	keyboardCodeMap := make(map[string][]int64)
	if err = loadKeyboardCodes(keyboardCodeMap, lookup, keyboardCodesPosixH, 0); err != nil {
		return nil, err
	}
	if err = loadKeyboardCodes(keyboardCodeMap, lookup, keyboardCodesWinH, 1); err != nil {
		return nil, err
	}
	return keyboardCodeMap, nil
}

var keyboardCodeRE = regexp.MustCompile(`(?m)^\s+(VKEY_.+?)\s+=\s+(.+?),`)

// loadKeyboardCodes loads the enum definition from the specified path, saving
// the resolved symbol value to the specified position for the resulting dom
// key name in the vkeyCodeMap.
func loadKeyboardCodes(vkeyCodeMap map[string][]int64, lookup map[string]string, path string, pos int) error {
	buf, err := grab(path)
	if err != nil {
		return err
	}
	buf = extract(buf, "KeyboardCode")
	matches := keyboardCodeRE.FindAllStringSubmatch(string(buf), -1)
	for _, m := range matches {
		v := m[2]
		switch {
		case strings.HasPrefix(m[2], "'"):
			v = fmt.Sprintf("0x%04x", m[2][1])
		case !strings.HasPrefix(m[2], "0x") && m[2] != "0":
			z, ok := lookup[v]
			if !ok {
				panic(fmt.Sprintf("could not find %s in lookup", v))
			}
			v = z
		}
		// load the value
		i, err := strconv.ParseInt(v, 0, 32)
		if err != nil {
			panic(fmt.Sprintf("could not parse %s // %s // %s", m[1], m[2], v))
		}
		vkey, ok := vkeyCodeMap[m[1]]
		if !ok {
			vkey = make([]int64, 2)
		}
		vkey[pos] = i
		vkeyCodeMap[m[1]] = vkey
	}
	return nil
}

var endRE = regexp.MustCompile(`\n}`)

// extract extracts a block of next from a block of c++ code.
func extract(buf []byte, name string) []byte {
	extractRE := regexp.MustCompile(`\s+` + name + `.+?{`)
	buf = buf[extractRE.FindIndex(buf)[0]:]
	return buf[:endRE.FindIndex(buf)[1]]
}

// grab retrieves a file from the chromium source code.
func grab(path string) ([]byte, error) {
	res, err := http.Get(path)
	if err != nil {
		return nil, fmt.Errorf("unable to get %s: %w", path, err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", path, err)
	}
	buf, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("unable to base64 decode %s: %w\n>>>\n%s\n<<<", path, err, string(body))
	}
	return buf, nil
}

var goCodes = map[rune]string{
	'\a': `\a`,
	'\b': `\b`,
	'\f': `\f`,
	'\n': `\n`,
	'\r': `\r`,
	'\t': `\t`,
	'\v': `\v`,
	'\\': `\\`,
	'\'': `\'`,
}

const (
	// chromiumSrc is the base chromium source repo location
	chromiumSrc = "https://chromium.googlesource.com/chromium/src/+/master/"
	// domUsLayoutDataH contains the {printable,non-printable} DomCode -> DomKey
	// also contains DomKey -> VKEY (not used)
	domUsLayoutDataH = chromiumSrc + "ui/events/keycodes/dom_us_layout_data.h?format=TEXT"
	// domCodeDataInc contains DomKey -> Key Name
	domCodeDataInc = chromiumSrc + "ui/events/keycodes/dom/dom_code_data.inc?format=TEXT"
	// domKeyDataInc contains DomKey -> Key Name + unicode (non-printable)
	domKeyDataInc = chromiumSrc + "ui/events/keycodes/dom/dom_key_data.inc?format=TEXT"
	// keyboardCodesPosixH contains the scan code definitions for posix (ie native) keys.
	keyboardCodesPosixH = chromiumSrc + "ui/events/keycodes/keyboard_codes_posix.h?format=TEXT"
	// keyboardCodesWinH contains the scan code definitions for windows keys.
	keyboardCodesWinH = chromiumSrc + "ui/events/keycodes/keyboard_codes_win.h?format=TEXT"
	// windowsKeyboardCodesH contains the actual #defs for windows.
	windowsKeyboardCodesH = chromiumSrc + "third_party/blink/renderer/platform/windows_keyboard_codes.h?format=TEXT"
)

type Key struct {
	// Code is the key code:
	// 								"Enter"     | "Comma"     | "KeyA"     | "MediaStop"
	Code string
	// Key is the key value:
	// 								"Enter"     | ","   "<"   | "a"   "A"  | "MediaStop"
	Key string
	// Text is the text for printable keys:
	// 								"\r"  "\r"  | ","   "<"   | "a"   "A"  | ""
	Text string
	// Unmodified is the unmodified text for printable keys:
	// 								"\r"  "\r"  | ","   ","   | "a"   "a"  | ""
	Unmodified string
	// Native is the native scan code.
	// 								0x13  0x13  | 0xbc  0xbc  | 0x61  0x41 | 0x00ae
	Native int64
	// Windows is the windows scan code.
	// 								0x13  0x13  | 0xbc  0xbc  | 0x61  0x41 | 0xe024
	Windows int64
	// Shift indicates whether or not the Shift modifier should be sent.
	// 								false false | false true  | false true | false
	Shift bool
	// Print indicates whether or not the character is a printable character
	// (ie, should a "char" event be generated).
	// 								true  true  | true  true  | true  true | false
	Print bool
}

const hdr = `// Package kb provides keyboard mappings for Chrome DOM Keys for use with input
// events.
package kb

` + `// Generated by gen.go. DO NOT EDIT.` + `

//go:generate go run gen.go

import (
	"runtime"
	"unicode"

	"github.com/chromedp/cdproto/input"
)

// Key contains information for generating a key press based off the unicode
// value.
//
// Example data for the following runes:
// 									'\r'  '\n'  | ','  '<'    | 'a'   'A'  | '\u0a07'
// 									_____________________________________________________
type Key struct {
	// Code is the key code:
	// 								"Enter"     | "Comma"     | "KeyA"     | "MediaStop"
	Code string
	// Key is the key value:
	// 								"Enter"     | ","   "<"   | "a"   "A"  | "MediaStop"
	Key string
	// Text is the text for printable keys:
	// 								"\r"  "\r"  | ","   "<"   | "a"   "A"  | ""
	Text string
	// Unmodified is the unmodified text for printable keys:
	// 								"\r"  "\r"  | ","   ","   | "a"   "a"  | ""
	Unmodified string
	// Native is the native scan code.
	// 								0x13  0x13  | 0xbc  0xbc  | 0x61  0x41 | 0x00ae
	Native int64
	// Windows is the windows scan code.
	// 								0x13  0x13  | 0xbc  0xbc  | 0x61  0x41 | 0xe024
	Windows int64
	// Shift indicates whether or not the Shift modifier should be sent.
	// 								false false | false true  | false true | false
	Shift bool
	// Print indicates whether or not the character is a printable character
	// (ie, should a "char" event be generated).
	// 								true  true  | true  true  | true  true | false
	Print bool
}

// EncodeUnidentified encodes a keyDown, char, and keyUp sequence for an
// unidentified rune.
func EncodeUnidentified(r rune) []*input.DispatchKeyEventParams {
	// create
	keyDown := input.DispatchKeyEventParams{
		Key: "Unidentified",
		/*NativeVirtualKeyCode:  int64(r), // not sure if should be specifying the key code or not ...
		WindowsVirtualKeyCode: int64(r),*/
	}
	keyUp := keyDown
	keyDown.Type, keyUp.Type = input.KeyDown, input.KeyUp
	// printable, so create char event
	if unicode.IsPrint(r) {
		keyChar := keyDown
		keyChar.Type = input.KeyChar
		keyChar.Text = string(r)
		keyChar.UnmodifiedText = string(r)

		return []*input.DispatchKeyEventParams{&keyDown, &keyChar, &keyUp}
	}
	return []*input.DispatchKeyEventParams{&keyDown, &keyUp}
}

// Encode encodes a keyDown, char, and keyUp sequence for the specified rune.
func Encode(r rune) []*input.DispatchKeyEventParams {
	// force \n -> \r
	if r == '\n' {
		r = '\r'
	}
	// if not known key, encode as unidentified
	v, ok := Keys[r]
	if !ok {
		return EncodeUnidentified(r)
	}
	// create
	keyDown := input.DispatchKeyEventParams{
		Key:                   v.Key,
		Code:                  v.Code,
		NativeVirtualKeyCode:  v.Native,
		WindowsVirtualKeyCode: v.Windows,
	}
	if runtime.GOOS == "darwin" {
		keyDown.NativeVirtualKeyCode = 0
	}
	if v.Shift {
		keyDown.Modifiers |= input.ModifierShift
	}
	keyUp := keyDown
	keyDown.Type, keyUp.Type = input.KeyDown, input.KeyUp
	// printable, so create char event
	if v.Print {
		keyChar := keyDown
		keyChar.Type = input.KeyChar
		keyChar.Text = v.Text
		keyChar.UnmodifiedText = v.Unmodified
		// the virtual key code for char events for printable characters will
		// be different than the defined keycode when not shifted...
		//
		// specifically, it always sends the ascii value as the scan code,
		// which is available as the rune.
		keyChar.NativeVirtualKeyCode = int64(r)
		keyChar.WindowsVirtualKeyCode = int64(r)
		return []*input.DispatchKeyEventParams{&keyDown, &keyChar, &keyUp}
	}
	return []*input.DispatchKeyEventParams{&keyDown, &keyUp}
}

// DOM keys.
const (
	%s)

// Keys is the map of unicode characters to their DOM key data.
var Keys = map[rune]*Key{
	%s}
`
