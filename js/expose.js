function deliverError(name, seq, message, stack) {
	const error = new Error(message);
	error.stack = stack;
	window["CDP_BINDING_" + name].callbacks.get(seq).reject(error);
	window["CDP_BINDING_" + name].callbacks.delete(seq);
}

function deliverResult(name, seq, result) {
	window["CDP_BINDING_" + name].callbacks.get(seq).resolve(result);
	window["CDP_BINDING_" + name].callbacks.delete(seq);
}

function addTargetBinding(type, name) {
	// This is the CDP binding.
	window["CDP_BINDING_" + name] = window[name];
	
	// We replace the CDP binding with a chromedp binding.
	Object.assign(window, {
		[name](args) {
			if(typeof args != "string"){
				return Promise.reject(new Error('function takes exactly one argument, this argument should be string'))
			}

			// This is the chromedp binding.
			const callChromedp = window["CDP_BINDING_" + name];

			if (callChromedp.callbacks == undefined) {
				callChromedp.callbacks = new Map()
			}
			if (callChromedp.lastSeq == undefined) {
				callChromedp.lastSeq = 0
			}

			const seq = callChromedp.lastSeq + 1
			callChromedp.lastSeq = seq;
			
			callChromedp(JSON.stringify({ type, name, seq, args }));

			return new Promise((resolve, reject) => {
				callChromedp.callbacks.set(seq, { resolve, reject });
			});
		},
	});
}

