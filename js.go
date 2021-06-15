package chromedp

const (
	// textJS is a javascript snippet that returns the innerText of the specified
	// visible (ie, offsetWidth || offsetHeight || getClientRects().length ) element.
	textJS = `function text() {
		if (this.offsetWidth || this.offsetHeight || this.getClientRects().length) {
			return this.innerText;
		}
		return '';
	}`

	// textContentJS is a javascript snippet that returns the textContent of the
	// specified element.
	textContentJS = `function textContent() {
		return this.textContent;
	}`

	// blurJS is a javascript snippet that blurs the specified element.
	blurJS = `function blur() {
		this.blur();
		return true;
	}`

	// submitJS is a javascript snippet that will call the containing form's
	// submit function, returning true or false if the call was successful.
	submitJS = `function submit() {
		if (this.nodeName === 'FORM') {
			HTMLFormElement.prototype.submit.call(this);
			return true;
		} else if (this.form !== null) {
			HTMLFormElement.prototype.submit.call(this.form);
			return true;
		}
		return false;
	}`

	// resetJS is a javascript snippet that will call the containing form's
	// reset function, returning true or false if the call was successful.
	resetJS = `function reset() {
		if (this.nodeName === 'FORM') {
			HTMLFormElement.prototype.reset.call(this);
			return true;
		} else if (this.form !== null) {
			HTMLFormElement.prototype.reset.call(this.form);
			return true;
		}
		return false;
	}`

	// attributeJS is a javascript snippet that returns the attribute of a specified
	// node.
	attributeJS = `function attribute(n) {
		return this[n];
	}`

	// setAttributeJS is a javascript snippet that sets the value of the specified
	// node, and returns the value.
	setAttributeJS = `function setAttribute(n, v) {
		this[n] = v;
		if (n === 'value') {
			this.dispatchEvent(new Event('input', { bubbles: true }));
			this.dispatchEvent(new Event('change', { bubbles: true }));
		}
		return this[n];
	}`

	// visibleJS is a javascript snippet that returns true or false depending on if
	// the specified node's offsetWidth, offsetHeight or getClientRects().length is
	// not null.
	visibleJS = `function visible() {
		return Boolean( this.offsetWidth || this.offsetHeight || this.getClientRects().length );
	}`

	// getClientRectJS is a javascript snippet that returns the information about the
	// size of the specified node and its position relative to its owner document.
	getClientRectJS = `function getClientRect() {
		const e = this.getBoundingClientRect(),
		t = this.ownerDocument.documentElement.getBoundingClientRect();
		return {
			x: e.left - t.left,
			y: e.top - t.top,
			width: e.width,
			height: e.height,
		};
	}`

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
