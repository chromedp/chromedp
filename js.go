package chromedp

import (
	_ "embed"
)

var (
	// textJS is a JavaScript snippet that returns the innerText of the specified
	// visible (i.e., offsetWidth || offsetHeight || getClientRects().length ) element.
	//go:embed js/text.js
	textJS string

	// textContentJS is a JavaScript snippet that returns the textContent of the
	// specified element.
	//go:embed js/textContent.js
	textContentJS string

	// blurJS is a JavaScript snippet that blurs the specified element.
	//go:embed js/blur.js
	blurJS string

	// submitJS is a JavaScript snippet that will call the containing form's
	// submit function, returning true or false if the call was successful.
	//go:embed js/submit.js
	submitJS string

	// resetJS is a JavaScript snippet that will call the containing form's
	// reset function, returning true or false if the call was successful.
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

	// visibleJS is a JavaScript snippet that returns true or false depending on if
	// the specified node's offsetWidth, offsetHeight or getClientRects().length is
	// not null.
	//go:embed js/visible.js
	visibleJS string

	// getClientRectJS is a JavaScript snippet that returns the information about the
	// size of the specified node and its position relative to its owner document.
	//go:embed js/getClientRect.js
	getClientRectJS string

	// waitForPredicatePageFunction is a JavaScript snippet that runs the polling in the
	// browser. It's copied from puppeteer. See
	// https://github.com/puppeteer/puppeteer/blob/669f04a7a6e96cc8353a8cb152898edbc25e7c15/src/common/DOMWorld.ts#L870-L944
	// It's modified to make mutation polling respect timeout even when there is not a DOM mutation.
	//go:embed js/waitForPredicatePageFunction.js
	waitForPredicatePageFunction string
)
