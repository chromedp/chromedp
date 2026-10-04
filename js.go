package chromedp

import (
	_ "embed"
)

var (
	// textJS is a JavaScript snippet that returns the innerText of the specified
	// visible (that is, offsetWidth || offsetHeight || getClientRects().length ) element.
	//go:embed js/text.js
	textJS string

	// textContentJS is a JavaScript snippet that returns the textContent of the
	// specified element.
	//go:embed js/textContent.js
	textContentJS string

	// blurJS is a JavaScript snippet that blurs the specified element.
	//go:embed js/blur.js
	blurJS string

	// submitJS is a JavaScript snippet that calls the submit function of the
	// form that contains the element. It returns true if the call succeeded.
	//go:embed js/submit.js
	submitJS string

	// resetJS is a JavaScript snippet that calls the reset function of the form
	// that contains the element. It returns true if the call succeeded.
	//go:embed js/reset.js
	resetJS string

	// attributeJS is a JavaScript snippet that returns the attribute of a specified
	// node.
	//go:embed js/attribute.js
	attributeJS string

	// setAttributeJS is a JavaScript snippet that sets the value of the specified
	// node, and returns the value.
	//go:embed js/setAttribute.js
	setAttributeJS string

	// visibleJS is a JavaScript snippet that returns true when the offsetWidth,
	// the offsetHeight, or getClientRects().length of the node is not null, and
	// false otherwise.
	//go:embed js/visible.js
	visibleJS string

	// getClientRectJS is a JavaScript snippet that returns the information about the
	// size of the specified node and its position relative to its owner document.
	//go:embed js/getClientRect.js
	getClientRectJS string

	// exposeFuncJS is a JavaScript function that defines window[name]. A call
	// of that function sends its arguments to the binding and returns a
	// promise. The Go side settles the promise with window[binding + "_reply"].
	//go:embed js/exposeFunc.js
	exposeFuncJS string

	// waitForPredicatePageFunction is a JavaScript snippet that runs the
	// polling in the browser. It is copied from puppeteer. See
	// https://github.com/puppeteer/puppeteer/blob/669f04a7a6e96cc8353a8cb152898edbc25e7c15/src/common/DOMWorld.ts#L870-L944
	// It is modified so that mutation polling respects the timeout even when the DOM does not mutate.
	//go:embed js/waitForPredicatePageFunction.js
	waitForPredicatePageFunction string
)
