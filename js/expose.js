function deliverError(name, seq, message, stack) {
	const error = new Error(message);
	error.stack = stack;
	window[name].callbacks.get(seq).reject(error);
	window[name].callbacks.delete(seq);
}

function deliverResult(name, seq, result) {
	window[name].callbacks.get(seq).resolve(result);
	window[name].callbacks.delete(seq);
}

function addTargetBinding(type, name) {
	// This is the CDP binding.
	const callCDP = window[name];

	// We replace the CDP binding with a chromedp binding.
	Object.assign(window, {
		[name](args) {
			if(typeof args != "string"){
				return Promise.reject(new Error('function takes exactly one argument, this argument should be string'))
			}
			var _a, _b;
			// This is the chromedp binding.
			const callChromedp = window[name];
			(_a = callChromedp.callbacks) !== null && _a !== void 0 ? _a : (callChromedp.callbacks = new Map());
			const seq = ((_b = callChromedp.lastSeq) !== null && _b !== void 0 ? _b : 0) + 1;
			callChromedp.lastSeq = seq;
			callCDP(JSON.stringify({ type, name, seq, args }));
			return new Promise((resolve, reject) => {
				callChromedp.callbacks.set(seq, { resolve, reject });
			});
		},
	});
}