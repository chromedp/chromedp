function exposeFunc(name, binding) {
    var reply = binding + '_reply';
    var send = window[binding];
    if (Object.prototype.hasOwnProperty.call(window, reply) || typeof send !== 'function') {
        return;
    }
    var pending = new Map();
    var last = 0;
    var fn = function () {
        var args = Array.prototype.slice.call(arguments);
        return new Promise(function (resolve, reject) {
            var id = ++last;
            pending.set(id, {resolve: resolve, reject: reject});
            try {
                send(JSON.stringify({id: id, args: args}));
            } catch (e) {
                pending.delete(id);
                reject(e);
            }
        });
    };
    // Define the property, and do not assign it, so that a name such as
    // __proto__ or a name with a setter on window does not act on the page.
    // The reply func comes last. It marks the script as done, so a script that
    // fails here runs again, and the error reaches ExposeFunc.
    Object.defineProperty(window, name, {
        value: fn,
        writable: true,
        configurable: true,
        enumerable: true,
    });
    Object.defineProperty(window, reply, {
        value: function (id, ok, value) {
            var call = pending.get(id);
            if (!call) {
                return;
            }
            pending.delete(id);
            if (ok) {
                call.resolve(value);
            } else {
                call.reject(new Error(value));
            }
        },
    });
}
