package chromedp

import (
	"fmt"

	"github.com/chromedp/cdproto/cdp"
)

const (
	// textJS is a javascript snippet that returns the concatenated innerText of all
	// visible (ie, offsetWidth || offsetHeight || getClientRects().length ) children.
	textJS = `(function(a) {
		var s = '';
		for (var i = 0; i < a.length; i++) {
			if (a[i].offsetWidth || a[i].offsetHeight || a[i].getClientRects().length) {
				s += a[i].innerText;
			}
		}
		return s;
	})(%s)`

	// textContentJS is a javascript snippet that returns the concatenated textContent
	// of all children.
	textContentJS = `(function(a) {
		var s = '';
		for (var i = 0; i < a.length; i++) {
			s += a[i].textContent;
		}
		return s;
	})(%s)`

	// blurJS is a javscript snippet that blurs the specified element.
	blurJS = `(function(a) {
		a.blur();
		return true;
	})(%s)`

	// scrollIntoViewJS is a javascript snippet that scrolls the specified node
	// into the window's viewport (if needed), returning the actual window x/y
	// after execution.
	scrollIntoViewJS = `(function(a) {
		a.scrollIntoViewIfNeeded(true);
		return [window.scrollX, window.scrollY];
	})(%s)`

	// submitJS is a javascript snippet that will call the containing form's
	// submit function, returning true or false if the call was successful.
	submitJS = `(function(a) {
		if (a.nodeName === 'FORM') {
			HTMLFormElement.prototype.submit.call(a);
			return true;
		} else if (a.form !== null) {
			HTMLFormElement.prototype.submit.call(a.form);
			return true;
		}
		return false;
	})(%s)`

	// resetJS is a javascript snippet that will call the containing form's
	// reset function, returning true or false if the call was successful.
	resetJS = `(function(a) {
		if (a.nodeName === 'FORM') {
			HTMLFormElement.prototype.reset.call(a);
			return true;
		} else if (a.form !== null) {
			HTMLFormElement.prototype.reset.call(a.form);
			return true;
		}
		return false;
	})(%s)`

	// attributeJS is a javascript snippet that returns the attribute of a specified
	// node.
	attributeJS = `(function(a, n) {
		return a[n];
	})(%s, %q)`

	// setAttributeJS is a javascript snippet that sets the value of the specified
	// node, and returns the value.
	setAttributeJS = `(function(a, n, v) {
		return a[n] = v;
	})(%s, %q, %q)`

	// visibleJS is a javascript snippet that returns true or false depending on if
	// the specified node's offsetWidth, offsetHeight or getClientRects().length is
	// not null.
	visibleJS = `(function(a) {
		return Boolean( a.offsetWidth || a.offsetHeight || a.getClientRects().length );
	})(%s)`

	// waitForPredicatePageFunction is a javascript snippet that runs the polling in the
	// browser. It's copied from puppeteer. See
	// https://github.com/puppeteer/puppeteer/blob/669f04a7a6e96cc8353a8cb152898edbc25e7c15/src/common/DOMWorld.ts#L870-L944
	// It's modified to make mutation polling respect timeout even when there is not DOM mutation.
	waitForPredicatePageFunction = `async function waitForPredicatePageFunction(predicateBody, polling, timeout, ...args) {
		const predicate = new Function('...args', predicateBody);
		let timedOut = false;
		if (timeout)
			setTimeout(() => (timedOut = true), timeout);
		if (polling === 'raf')
			return await pollRaf();
		if (polling === 'mutation')
			return await pollMutation();
		if (typeof polling === 'number')
			return await pollInterval(polling);
		/**
		 * @returns {!Promise<*>}
		 */
		async function pollMutation() {
			const success = await predicate(...args);
			if (success)
				return Promise.resolve(success);
			let fulfill;
			const result = new Promise((x) => (fulfill = x));
			const observer = new MutationObserver(async () => {
				if (timedOut) {
					observer.disconnect();
					fulfill();
				}
				const success = await predicate(...args);
				if (success) {
					observer.disconnect();
					fulfill(success);
				}
			});
			observer.observe(document, {
				childList: true,
				subtree: true,
				attributes: true,
			});
			if (timeout)
				setTimeout(() => {
					observer.disconnect();
					fulfill();
				}, timeout);
			return result;
		}
		async function pollRaf() {
			let fulfill;
			const result = new Promise((x) => (fulfill = x));
			await onRaf();
			return result;
			async function onRaf() {
				if (timedOut) {
					fulfill();
					return;
				}
				const success = await predicate(...args);
				if (success)
					fulfill(success);
				else
					requestAnimationFrame(onRaf);
			}
		}
		async function pollInterval(pollInterval) {
			let fulfill;
			const result = new Promise((x) => (fulfill = x));
			await onTimeout();
			return result;
			async function onTimeout() {
				if (timedOut) {
					fulfill();
					return;
				}
				const success = await predicate(...args);
				if (success)
					fulfill(success);
				else
					setTimeout(onTimeout, pollInterval);
			}
		}
	}`
)

// snippet builds a Javascript expression snippet.
func snippet(js string, f func(n *cdp.Node) string, sel interface{}, n *cdp.Node, v ...interface{}) string {
	switch s := sel.(type) {
	case *Selector:
		if s != nil && s.raw {
			return fmt.Sprintf(js, append([]interface{}{s.selAsString()}, v...)...)
		}
	}
	return fmt.Sprintf(js, append([]interface{}{f(n)}, v...)...)
}

// cashX returns the $x() expression using the node's full xpath value.
func cashX(flatten bool) func(*cdp.Node) string {
	return func(n *cdp.Node) string {
		if flatten {
			return fmt.Sprintf(`$x(%q)[0]`, n.PartialXPath())
		}
		return fmt.Sprintf(`$x(%q)`, n.PartialXPath())
	}
}

// cashXNode returns the $x(/node()) expression using the node's full xpath value.
func cashXNode(flatten bool) func(*cdp.Node) string {
	return func(n *cdp.Node) string {
		if flatten {
			return fmt.Sprintf(`$x(%q)[0]`, n.PartialXPath()+"/node()")
		}
		return fmt.Sprintf(`$x(%q)`, n.PartialXPath()+"/node()")
	}
}
