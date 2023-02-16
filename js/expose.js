var chromedpExposeFunc = chromedpExposeFunc || {
  bindings: {},
  deliverError: function (name, seq, message) {
    const error = new Error(message);
    chromedpExposeFunc.bindings[name].callbacks.get(seq).reject(error);
    chromedpExposeFunc.bindings[name].callbacks.delete(seq);
  },
  deliverResult: function (name, seq, result) {
    chromedpExposeFunc.bindings[name].callbacks.get(seq).resolve(result);
    chromedpExposeFunc.bindings[name].callbacks.delete(seq);
  },
  wrapBinding: function (type, name) {
    // Store the binding function added by the call of runtime.AddBinding.
    chromedpExposeFunc.bindings[name] = window[name];

    // Replace the binding function.
    Object.assign(window, {
      [name](args) {
        if (typeof args != 'string') {
          return Promise.reject(
            new Error(
              'function takes exactly one argument, this argument should be string'
            )
          );
        }

        const binding = chromedpExposeFunc.bindings[name];

        binding.callbacks ??= new Map();

        const seq = (binding.lastSeq ?? 0) + 1;
        binding.lastSeq = seq;

        // Call the binding function to trigger runtime.EventBindingCalled.
        binding(JSON.stringify({ type, name, seq, args }));

        return new Promise((resolve, reject) => {
          binding.callbacks.set(seq, { resolve, reject });
        });
      },
    });
  },
};
