package chromedp

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
)

// forceIP tries to force the host component in urlstr to be an IP address.
//
// Since Chrome 66+, Chrome DevTools Protocol clients connecting to a browser
// must send the "Host:" header as either an IP address, or "localhost".
func forceIP(urlstr string) string {
	u, err := url.Parse(urlstr)
	if err != nil {
		return urlstr
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return urlstr
	}
	addr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return urlstr
	}
	u.Host = net.JoinHostPort(addr.IP.String(), port)
	return u.String()
}

// detectURL detects the websocket debugger URL if the provided URL is not a
// valid websocket debugger URL.
//
// A valid websocket debugger URL is something like:
// ws://127.0.0.1:9222/devtools/browser/...
// The original URL with the following formats are accepted:
// * ws://127.0.0.1:9222/
// * http://127.0.0.1:9222/
func detectURL(urlstr string) string {
	if strings.Contains(urlstr, "/devtools/browser/") {
		return urlstr
	}

	// replace the scheme and path to construct the URL like:
	// http://127.0.0.1:9222/json/version
	u, err := url.Parse(urlstr)
	if err != nil {
		return urlstr
	}
	u.Scheme = "http"
	u.Path = "/json/version"

	// to get "webSocketDebuggerUrl" in the response
	resp, err := http.Get(forceIP(u.String()))
	if err != nil {
		return urlstr
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return urlstr
	}
	// the browser will construct the debugger URL using the "host" header of the /json/version request.
	// for example, run headless-shell in a container: docker run -d -p 9000:9222 chromedp/headless-shell:latest
	// then: curl http://127.0.0.1:9000/json/version
	// and the debugger URL will be something like: ws://127.0.0.1:9000/devtools/browser/...
	wsURL := result["webSocketDebuggerUrl"].(string)
	return wsURL
}

func runListeners(list []cancelableListener, ev interface{}) []cancelableListener {
	for i := 0; i < len(list); {
		listener := list[i]
		select {
		case <-listener.ctx.Done():
			list = append(list[:i], list[i+1:]...)
			continue
		default:
			listener.fn(ev)
			i++
		}
	}
	return list
}

// frameOp is a frame manipulation operation.
type frameOp func(*cdp.Frame)

func frameAttached(id cdp.FrameID) frameOp {
	return func(f *cdp.Frame) {
		f.ParentID = id
		setFrameState(f, cdp.FrameAttached)
	}
}

func frameDetached(f *cdp.Frame) {
	f.ParentID = cdp.EmptyFrameID
	clearFrameState(f, cdp.FrameAttached)
}

func frameStartedLoading(f *cdp.Frame) {
	setFrameState(f, cdp.FrameLoading)
}

func frameStoppedLoading(f *cdp.Frame) {
	clearFrameState(f, cdp.FrameLoading)
}

// setFrameState sets the frame state via bitwise or (|).
func setFrameState(f *cdp.Frame, fs cdp.FrameState) {
	f.State |= fs
}

// clearFrameState clears the frame state via bit clear (&^).
func clearFrameState(f *cdp.Frame, fs cdp.FrameState) {
	f.State &^= fs
}

// nodeOp is a node manipulation operation.
type nodeOp func(*cdp.Node)

func walk(m map[cdp.NodeID]*cdp.Node, n *cdp.Node) {
	n.RLock()
	defer n.RUnlock()
	m[n.NodeID] = n

	for _, c := range n.Children {
		c.Lock()
		c.Parent = n
		c.Invalidated = n.Invalidated
		c.Unlock()

		walk(m, c)
	}

	for _, c := range n.ShadowRoots {
		c.Lock()
		c.Parent = n
		c.Invalidated = n.Invalidated
		c.Unlock()

		walk(m, c)
	}

	for _, c := range n.PseudoElements {
		c.Lock()
		c.Parent = n
		c.Invalidated = n.Invalidated
		c.Unlock()

		walk(m, c)
	}

	for _, c := range []*cdp.Node{n.ContentDocument, n.TemplateContent} {
		if c == nil {
			continue
		}

		c.Lock()
		c.Parent = n
		c.Invalidated = n.Invalidated
		c.Unlock()

		walk(m, c)
	}
}

func setChildNodes(m map[cdp.NodeID]*cdp.Node, nodes []*cdp.Node) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		n.Children = nodes
		n.Unlock()

		walk(m, n)
	}
}

func attributeModified(name, value string) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		var found bool
		var i int
		for ; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == name {
				found = true
				break
			}
		}

		if found {
			n.Attributes[i] = name
			n.Attributes[i+1] = value
		} else {
			n.Attributes = append(n.Attributes, name, value)
		}
	}
}

func attributeRemoved(name string) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		var a []string
		for i := 0; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == name {
				continue
			}
			a = append(a, n.Attributes[i], n.Attributes[i+1])
		}
		n.Attributes = a
	}
}

func inlineStyleInvalidated(ids []cdp.NodeID) nodeOp {
	return func(n *cdp.Node) {
	}
}

func characterDataModified(characterData string) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.Value = characterData
	}
}

func childNodeCountUpdated(count int64) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.ChildNodeCount = count
	}
}

func childNodeInserted(m map[cdp.NodeID]*cdp.Node, prevID cdp.NodeID, c *cdp.Node) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		n.Children = insertNode(n.Children, prevID, c)
		n.Unlock()

		walk(m, n)
	}
}

func childNodeRemoved(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.Children = removeNode(n.Children, id)
		delete(m, id)
	}
}

func shadowRootPushed(m map[cdp.NodeID]*cdp.Node, c *cdp.Node) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		n.ShadowRoots = append(n.ShadowRoots, c)
		n.Unlock()

		walk(m, n)
	}
}

func shadowRootPopped(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.ShadowRoots = removeNode(n.ShadowRoots, id)
		delete(m, id)
	}
}

func pseudoElementAdded(m map[cdp.NodeID]*cdp.Node, c *cdp.Node) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		n.PseudoElements = append(n.PseudoElements, c)
		n.Unlock()

		walk(m, n)
	}
}

func pseudoElementRemoved(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.PseudoElements = removeNode(n.PseudoElements, id)
		delete(m, id)
	}
}

func distributedNodesUpdated(nodes []*cdp.BackendNode) nodeOp {
	return func(n *cdp.Node) {
		n.Lock()
		defer n.Unlock()

		n.DistributedNodes = nodes
	}
}

func insertNode(n []*cdp.Node, prevID cdp.NodeID, c *cdp.Node) []*cdp.Node {
	var i int
	var found bool
	for ; i < len(n); i++ {
		if n[i].NodeID == prevID {
			found = true
			break
		}
	}

	if !found {
		return append([]*cdp.Node{c}, n...)
	}

	i++
	n = append(n, nil)
	copy(n[i+1:], n[i:])
	n[i] = c

	return n
}

func removeNode(n []*cdp.Node, id cdp.NodeID) []*cdp.Node {
	if len(n) == 0 {
		return n
	}

	var found bool
	var i int
	for ; i < len(n); i++ {
		if n[i].NodeID == id {
			found = true
			break
		}
	}

	if !found {
		return n
	}

	return append(n[:i], n[i+1:]...)
}

// isCouldNotComputeBoxModelError unwraps err as a MessageError and determines
// if it is a compute box model error.
func isCouldNotComputeBoxModelError(err error) bool {
	e, ok := err.(*cdproto.Error)
	return ok && e.Code == -32000 && e.Message == "Could not compute box model."
}
