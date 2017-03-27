// DO NOT EDIT. Generated by github.com/relab/gorums/cmd/gentemplates
// Template source files to edit is in the 'dev' folder.

package gorums

const calltype_common_definitions_tmpl = `{{/* Remember to run 'make goldenanddev' after editing this file. */}}
{{/* calltype_common_definitions.tmpl will only be executed for each 'calltype' template. */}}

{{define "callGRPC"}}
func callGRPC{{.MethodName}}(ctx context.Context, node *Node, arg *{{.FQReqName}}, replyChan chan<- {{.UnexportedTypeName}}, errChan chan<- CallGRPCError) {
	reply := new({{.FQRespName}})
	start := time.Now()
	err := grpc.Invoke(
		ctx,
		"/{{.ServPackageName}}.{{.ServName}}/{{.MethodName}}",
		arg,
		reply,
		node.conn,
	)
	switch grpc.Code(err) { // nil -> codes.OK
	case codes.OK, codes.Canceled:
		node.setLatency(time.Since(start))
	default:
		node.setLastErr(err)
    select {
    case errChan <- CallGRPCError{
         NodeID: node.ID(),
         Cause: err,
    }:
    default:
    }
	}
	replyChan <- {{.UnexportedTypeName}}{node.id, reply, err}
}
{{end}}

{{define "trace"}}
	var ti traceInfo
	if c.mgr.opts.trace {
		ti.tr = trace.New("gorums."+c.tstring()+".Sent", "{{.MethodName}}")
		defer ti.tr.Finish()

		ti.firstLine.cid = c.id
		if deadline, ok := ctx.Deadline(); ok {
			ti.firstLine.deadline = deadline.Sub(time.Now())
		}
		ti.tr.LazyLog(&ti.firstLine, false)
		ti.tr.LazyLog(&payload{sent: true, msg: a}, false)

		defer func() {
			ti.tr.LazyLog(&qcresult{
				ids:   resp.NodeIDs,
				reply: resp.{{.CustomRespName}},
				err:   resp.err,
			}, false)
			if resp.err != nil {
				ti.tr.SetError()
			}
		}()
	}
{{end}}

{{define "simple_trace"}}
	var ti traceInfo
	if c.mgr.opts.trace {
		ti.tr = trace.New("gorums."+c.tstring()+".Sent", "{{.MethodName}}")
		defer ti.tr.Finish()

		ti.firstLine.cid = c.id
		if deadline, ok := ctx.Deadline(); ok {
			ti.firstLine.deadline = deadline.Sub(time.Now())
		}
		ti.tr.LazyLog(&ti.firstLine, false)
		ti.tr.LazyLog(&payload{sent: true, msg: a}, false)

		defer func() {
			ti.tr.LazyLog(&qcresult{
				reply: resp,
				err:   err,
			}, false)
			if err != nil {
				ti.tr.SetError()
			}
		}()
	}
{{end}}

{{define "unexported_method_signature"}}
{{- if .PerNodeArg}}
func (c *Configuration) {{.UnexportedMethodName}}(ctx context.Context, a *{{.FQReqName}}, f func(arg {{.FQReqName}}, nodeID uint32) *{{.FQReqName}}, resp *{{.TypeName}}) {
{{- else}}
func (c *Configuration) {{.UnexportedMethodName}}(ctx context.Context, a *{{.FQReqName}}, resp *{{.TypeName}}) {
{{- end -}}
{{end}}

{{define "callLoop"}}
	replyChan := make(chan {{.UnexportedTypeName}}, c.n)
	for _, n := range c.nodes {
{{- if .PerNodeArg}}
		go callGRPC{{.MethodName}}(ctx, n, f(*a, n.id), replyChan, c.errs)
{{else}}
		go callGRPC{{.MethodName}}(ctx, n, a, replyChan, c.errs)
{{end -}}
	}
{{end}}
`

const calltype_correctable_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{- if not .IgnoreImports}}
package {{.PackageName}}

import (
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"golang.org/x/net/context")

{{- end}}

{{range $elm := .Services}}

{{if .Correctable}}

/* Exported types and methods for correctable method {{.MethodName}} */

// {{.TypeName}} is a reference to a correctable {{.MethodName}} quorum call.
type {{.TypeName}} struct {
	sync.Mutex
	// the actual reply
	*{{.FQCustomRespName}}
	NodeIDs  []uint32
	level    int
	err      error
	done     bool
	watchers []*struct {
		level int
		ch    chan struct{}
	}
	donech chan struct{}
}

// {{.MethodName}} asynchronously invokes a
// correctable {{.MethodName}} quorum call on configuration c and returns a
// {{.TypeName}} which can be used to inspect any replies or errors
// when available.
func (c *Configuration) {{.MethodName}}(ctx context.Context, args *{{.FQReqName}}) *{{.TypeName}} {
	corr := &{{.TypeName}}{
		level:   LevelNotSet,
		NodeIDs: make([]uint32, 0, c.n),
		donech:  make(chan struct{}),
	}
	go func() {
		c.{{.UnexportedMethodName}}(ctx, args, corr)
	}()
	return corr
}

// Get returns the reply, level and any error associated with the
// {{.MethodName}}. The method does not block until a (possibly
// itermidiate) reply or error is available. Level is set to LevelNotSet if no
// reply has yet been received. The Done or Watch methods should be used to
// ensure that a reply is available.
func (c *{{.TypeName}}) Get() (*{{.FQCustomRespName}}, int, error) {
	c.Lock()
	defer c.Unlock()
	return c.{{.CustomRespName}}, c.level, c.err
}

// Done returns a channel that's closed when the correctable {{.MethodName}}
// quorum call is done. A call is considered done when the quorum function has
// signaled that a quorum of replies was received or that the call returned an
// error.
func (c *{{.TypeName}}) Done() <-chan struct{} {
	return c.donech
}

// Watch returns a channel that's closed when a reply or error at or above the
// specified level is available. If the call is done, the channel is closed
// disregardless of the specified level.
func (c *{{.TypeName}}) Watch(level int) <-chan struct{} {
	ch := make(chan struct{})
	c.Lock()
	if level < c.level {
		close(ch)
		c.Unlock()
		return ch
	}
	c.watchers = append(c.watchers, &struct {
		level int
		ch    chan struct{}
	}{level, ch})
	c.Unlock()
	return ch
}

func (c *{{.TypeName}}) set(reply *{{.FQCustomRespName}}, level int, err error, done bool) {
	c.Lock()
	if c.done {
		c.Unlock()
		panic("set(...) called on a done correctable")
	}
	c.{{.CustomRespName}}, c.level, c.err, c.done = reply, level, err, done
	if done {
		close(c.donech)
		for _, watcher := range c.watchers {
			if watcher != nil {
				close(watcher.ch)
			}
		}
		c.Unlock()
		return
	}
	for i := range c.watchers {
		if c.watchers[i] != nil && c.watchers[i].level <= level {
			close(c.watchers[i].ch)
			c.watchers[i] = nil
		}
	}
	c.Unlock()
}

/* Unexported types and methods for correctable method {{.MethodName}} */

type {{.UnexportedTypeName}} struct {
	nid   uint32
	reply *{{.FQRespName}}
	err   error
}

{{template "unexported_method_signature" . -}}
	{{- template "callLoop" .}}

	var (
		replyValues = make([]*{{.FQRespName}}, 0, c.n)
		clevel      = LevelNotSet
		reply		*{{.FQCustomRespName}}
		rlevel      int
		errCount    int
		quorum      bool
	)

	for {
		select {
		case r := <-replyChan:
			resp.NodeIDs = append(resp.NodeIDs, r.nid)
			if r.err != nil {
				errCount++
				break
			}
			replyValues = append(replyValues, r.reply)
{{- if .QFWithReq}}
			reply, rlevel, quorum = c.qspec.{{.MethodName}}QF(a, replyValues)
{{else}}
			reply, rlevel, quorum = c.qspec.{{.MethodName}}QF(replyValues)
{{end -}}
			if quorum {
				resp.set(reply, rlevel, nil, true)
				return
			}
			if rlevel > clevel {
				clevel = rlevel
				resp.set(reply, rlevel, nil, false)
			}
		case <-ctx.Done():
			resp.set(reply, clevel, QuorumCallError{ctx.Err().Error(), errCount, len(replyValues)}, true)
			return
		}

		if errCount+len(replyValues) == c.n {
			resp.set(reply, clevel, QuorumCallError{"incomplete call", errCount, len(replyValues)}, true)
			return
		}
	}
}

{{template "callGRPC" .}}

{{- end -}}
{{- end -}}
`

const calltype_correctable_prelim_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{- if not .IgnoreImports}}
package {{.PackageName}}

import (
	"io"
	"sync"

	"golang.org/x/net/context"
)
{{- end}}

{{range $elm := .Services}}

{{if .CorrectablePrelim}}

/* Exported types and methods for correctable prelim method {{.MethodName}} */

// {{.TypeName}} is a reference to a correctable quorum call
// with server side preliminary reply support.
type {{.TypeName}} struct {
	sync.Mutex
	// the actual reply
	*{{.FQCustomRespName}}
	NodeIDs  []uint32
	level    int
	err      error
	done     bool
	watchers []*struct {
		level int
		ch    chan struct{}
	}
	donech chan struct{}
}

// {{.MethodName}} asynchronously invokes a correctable {{.MethodName}} quorum call
// with server side preliminary reply support on configuration c and returns a
// {{.TypeName}} which can be used to inspect any replies or errors
// when available.
func (c *Configuration) {{.MethodName}}(ctx context.Context, args *{{.FQReqName}}) *{{.TypeName}} {
	corr := &{{.TypeName}}{
		level:  LevelNotSet,
		NodeIDs: make([]uint32, 0, c.n),
		donech: make(chan struct{}),
	}
	go func() {
		c.{{.UnexportedMethodName}}(ctx, args, corr)
	}()
	return corr
}

// Get returns the reply, level and any error associated with the
// {{.MethodName}}. The method does not block until a (possibly
// itermidiate) reply or error is available. Level is set to LevelNotSet if no
// reply has yet been received. The Done or Watch methods should be used to
// ensure that a reply is available.
func (c *{{.TypeName}}) Get() (*{{.FQCustomRespName}}, int, error) {
	c.Lock()
	defer c.Unlock()
	return c.{{.CustomRespName}}, c.level, c.err
}

// Done returns a channel that's closed when the correctable {{.MethodName}}
// quorum call is done. A call is considered done when the quorum function has
// signaled that a quorum of replies was received or that the call returned an
// error.
func (c *{{.TypeName}}) Done() <-chan struct{} {
	return c.donech
}

// Watch returns a channel that's closed when a reply or error at or above the
// specified level is available. If the call is done, the channel is closed
// disregardless of the specified level.
func (c *{{.TypeName}}) Watch(level int) <-chan struct{} {
	ch := make(chan struct{})
	c.Lock()
	if level < c.level {
		close(ch)
		c.Unlock()
		return ch
	}
	c.watchers = append(c.watchers, &struct {
		level int
		ch    chan struct{}
	}{level, ch})
	c.Unlock()
	return ch
}

func (c *{{.TypeName}}) set(reply *{{.FQCustomRespName}}, level int, err error, done bool) {
	c.Lock()
	if c.done {
		c.Unlock()
		panic("set(...) called on a done correctable")
	}
	c.{{.CustomRespName}}, c.level, c.err, c.done = reply, level, err, done
	if done {
		close(c.donech)
		for _, watcher := range c.watchers {
			if watcher != nil {
				close(watcher.ch)
			}
		}
		c.Unlock()
		return
	}
	for i := range c.watchers {
		if c.watchers[i] != nil && c.watchers[i].level <= level {
			close(c.watchers[i].ch)
			c.watchers[i] = nil
		}
	}
	c.Unlock()
}

/* Unexported types and methods for correctable prelim method {{.MethodName}} */

type {{.UnexportedTypeName}} struct {
	nid   uint32
	reply *{{.FQRespName}}
	err   error
}

{{template "unexported_method_signature" . -}}
	{{- template "callLoop" .}}

	var (
		replyValues = make([]*{{.FQRespName}}, 0, c.n*2)
		clevel      = LevelNotSet
		reply		*{{.FQCustomRespName}}
		rlevel      int
		errCount    int
		quorum      bool
	)

	for {
		select {
		case r := <-replyChan:
			resp.NodeIDs = appendIfNotPresent(resp.NodeIDs, r.nid)
			if r.err != nil {
				errCount++
				break
			}
			replyValues = append(replyValues, r.reply)
{{- if .QFWithReq}}
			reply, rlevel, quorum = c.qspec.{{.MethodName}}QF(a, replyValues)
{{else}}
			reply, rlevel, quorum = c.qspec.{{.MethodName}}QF(replyValues)
{{end -}}
			if quorum {
				resp.set(reply, rlevel, nil, true)
				return
			}
			if rlevel > clevel {
				clevel = rlevel
				resp.set(reply, rlevel, nil, false)
			}
		case <-ctx.Done():
			resp.set(reply, clevel, QuorumCallError{ctx.Err().Error(), errCount, len(replyValues)}, true)
			return
		}

		if errCount == c.n { // Can't rely on reply count.
			resp.set(reply, clevel, QuorumCallError{"incomplete call", errCount, len(replyValues)}, true)
			return
		}
	}
}

func callGRPC{{.MethodName}}(ctx context.Context, node *Node, arg *{{.FQReqName}}, replyChan chan<- {{.UnexportedTypeName}}, _ chan<- CallGRPCError) {
	x := New{{.ServName}}Client(node.conn)
	y, err := x.{{.MethodName}}(ctx, arg)
	if err != nil {
		replyChan <- {{.UnexportedTypeName}}{node.id, nil, err}
		return
	}

	for {
		reply, err := y.Recv()
		if err == io.EOF {
			return
		}
		replyChan <- {{.UnexportedTypeName}}{node.id, reply, err}
		if err != nil {
			return
		}
	}
}

{{- end -}}
{{- end -}}
`

const calltype_future_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{if not .IgnoreImports}}
package {{.PackageName}}

import (
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)
{{end}}

{{range $elm := .Services}}

{{if .Future}}

/* Exported types and methods for asynchronous quorum call method {{.MethodName}} */

// {{.TypeName}} is a future object for an asynchronous {{.MethodName}} quorum call invocation.
type {{.TypeName}} struct {
	// the actual reply
	*{{.FQCustomRespName}}
	NodeIDs  []uint32
	err   error
	c     chan struct{}
}

{{if .PerNodeArg}}

// {{.MethodName}} asynchronously invokes a quorum call on each node in
// configuration c, with the argument returned by the provided perNode
// function and returns the result as a {{.TypeName}}, which can be used
// to inspect the quorum call reply and error when available. 
// The perNode function takes the provided arg and returns a {{.FQReqName}}
// object to be passed to the given nodeID.
func (c *Configuration) {{.MethodName}}(ctx context.Context, arg *{{.FQReqName}}, perNode func(arg {{.FQReqName}}, nodeID uint32) *{{.FQReqName}}) *{{.TypeName}} {
	f := &{{.TypeName}}{
		NodeIDs: make([]uint32, 0, c.n),
		c:       make(chan struct{}, 1),
	}
	go func() {
		defer close(f.c)
		c.{{.UnexportedMethodName}}(ctx, arg, perNode, f)
	}()
	return f
}

{{else}}

// {{.MethodName}} asynchronously invokes a quorum call on configuration c
// and returns a {{.TypeName}} which can be used to inspect the quorum call
// reply and error when available.
func (c *Configuration) {{.MethodName}}(ctx context.Context, arg *{{.FQReqName}}) *{{.TypeName}} {
	f := &{{.TypeName}}{
		NodeIDs: make([]uint32, 0, c.n),
		c:       make(chan struct{}, 1),
	}
	go func() {
		defer close(f.c)
		c.{{.UnexportedMethodName}}(ctx, arg, f)
	}()
	return f
}

{{- end}}

// Get returns the reply and any error associated with the {{.MethodName}}.
// The method blocks until a reply or error is available.
func (f *{{.TypeName}}) Get() (*{{.FQCustomRespName}}, error) {
	<-f.c
	return f.{{.CustomRespName}}, f.err
}

// Done reports if a reply and/or error is available for the {{.MethodName}}.
func (f *{{.TypeName}}) Done() bool {
	select {
	case <-f.c:
		return true
	default:
		return false
	}
}

/* Unexported types and methods for asynchronous quorum call method {{.MethodName}} */

type {{.UnexportedTypeName}} struct {
	nid   uint32
	reply *{{.FQRespName}}
	err   error
}

{{template "unexported_method_signature" .}}
	{{- template "trace" .}}

	{{template "callLoop" .}}

	var (
		replyValues = make([]*{{.FQRespName}}, 0, c.n)
		reply		*{{.FQCustomRespName}}
		errCount    int
		quorum      bool
	)

	for {
		select {
		case r := <-replyChan:
			resp.NodeIDs = append(resp.NodeIDs, r.nid)
			if r.err != nil {
				errCount++
				break
			}
			if c.mgr.opts.trace {
				ti.tr.LazyLog(&payload{sent: false, id: r.nid, msg: r.reply}, false)
			}
			replyValues = append(replyValues, r.reply)
{{- if .QFWithReq}}
			if reply, quorum = c.qspec.{{.MethodName}}QF(a, replyValues); quorum {
{{else}}
			if reply, quorum = c.qspec.{{.MethodName}}QF(replyValues); quorum {
{{end -}}
				resp.{{.CustomRespName}}, resp.err = reply, nil
				return
			}
		case <-ctx.Done():
			resp.{{.CustomRespName}}, resp.err = reply, QuorumCallError{ctx.Err().Error(), errCount, len(replyValues)}
			return
		}

		if errCount+len(replyValues) == c.n {
			resp.{{.CustomRespName}}, resp.err = reply, QuorumCallError{"incomplete call", errCount, len(replyValues)}
			return
		}
	}
}

{{template "callGRPC" .}}

{{- end -}}
{{- end -}}
`

const calltype_multicast_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{if not .IgnoreImports}}
package {{.PackageName}}

import "golang.org/x/net/context"
{{end}}

{{range $elm := .Services}}

{{if .Multicast}}

/* Exported types and methods for multicast method {{.MethodName}} */

// {{.MethodName}} is a one-way multicast call on all nodes in configuration c,
// using the same argument arg. The call is asynchronous and has no return value.
func (c *Configuration) {{.MethodName}}(ctx context.Context, arg *{{.FQReqName}}) error {
	return c.{{.UnexportedMethodName}}(ctx, arg)
}

/* Unexported types and methods for multicast method {{.MethodName}} */

func (c *Configuration) {{.UnexportedMethodName}}(ctx context.Context, arg *{{.FQReqName}}) error {
	for _, node := range c.nodes {
		go func(n *Node) {
			err := n.{{.MethodName}}Client.Send(arg)
			if err == nil {
				return
			}
			if c.mgr.logger != nil {
				c.mgr.logger.Printf("%d: {{.UnexportedMethodName}} stream send error: %v", n.id, err)
			}
		}(node)
	}

	return nil
}
{{- end -}}
{{- end -}}
`

const calltype_quorumcall_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{if not .IgnoreImports}}
package {{.PackageName}}

import (
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)
{{end}}

{{range $elm := .Services}}

{{if .QuorumCall}}

/* Exported types and methods for quorum call method {{.MethodName}} */

{{if .PerNodeArg}}

// {{.MethodName}} is invoked as a quorum call on each node in configuration c,
// with the argument returned by the provided perNode function and returns the
// result. The perNode function takes a request arg and
// returns a {{.FQReqName}} object to be passed to the given nodeID.
func (c *Configuration) {{.MethodName}}(ctx context.Context, arg *{{.FQReqName}}, perNode func(arg {{.FQReqName}}, nodeID uint32) *{{.FQReqName}}) (*{{.FQCustomRespName}}, error) {
	return c.{{.UnexportedMethodName}}(ctx, arg, perNode)
}

{{else}}

// {{.MethodName}} is invoked as a quorum call on all nodes in configuration c,
// using the same argument arg, and returns the result.
func (c *Configuration) {{.MethodName}}(ctx context.Context, arg *{{.FQReqName}}) (*{{.FQCustomRespName}}, error) {
	return c.{{.UnexportedMethodName}}(ctx, arg)
}

{{- end}}

/* Unexported types and methods for quorum call method {{.MethodName}} */

type {{.UnexportedTypeName}} struct {
	nid   uint32
	reply *{{.FQRespName}}
	err   error
}

{{- if .PerNodeArg}}
func (c *Configuration) {{.UnexportedMethodName}}(ctx context.Context, a *{{.FQReqName}}, f func(arg {{.FQReqName}}, nodeID uint32) *{{.FQReqName}}) (resp *{{.FQCustomRespName}}, err error) {
{{- else}}
func (c *Configuration) {{.UnexportedMethodName}}(ctx context.Context, a *{{.FQReqName}}) (resp *{{.FQCustomRespName}}, err error) {
{{- end -}}
	{{- template "simple_trace" .}}

	{{template "callLoop" .}}

	var (
		replyValues = make([]*{{.FQRespName}}, 0, c.n)
		errCount    int
		quorum      bool
	)

	for {
		select {
		case r := <-replyChan:
			if r.err != nil {
				errCount++
				break
			}
			if c.mgr.opts.trace {
				ti.tr.LazyLog(&payload{sent: false, id: r.nid, msg: r.reply}, false)
			}
			replyValues = append(replyValues, r.reply)
{{- if .QFWithReq}}
			if resp, quorum = c.qspec.{{.MethodName}}QF(a, replyValues); quorum {
{{else}}
			if resp, quorum = c.qspec.{{.MethodName}}QF(replyValues); quorum {
{{end -}}
				return resp, nil
			}
		case <-ctx.Done():
			return resp, QuorumCallError{ctx.Err().Error(), errCount, len(replyValues)}
		}

		if errCount+len(replyValues) == c.n {
			return resp, QuorumCallError{"incomplete call", errCount, len(replyValues)}
		}
	}
}

{{template "callGRPC" .}}

{{- end -}}
{{- end -}}
`

const node_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{- if not .IgnoreImports}}
package {{.PackageName}}

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
)
{{- end}}

// Node encapsulates the state of a node on which a remote procedure call
// can be made.
type Node struct {
	// Only assigned at creation.
	id   uint32
	self bool
	addr string
	conn *grpc.ClientConn


{{range .Clients}}
	{{.}} {{.}}
{{end}}

{{range .Services}}
{{if .Multicast}}
	{{.MethodName}}Client {{.ServName}}_{{.MethodName}}Client
{{end}}
{{end}}

	sync.Mutex
	lastErr error
	latency time.Duration
}

func (n *Node) connect(opts ...grpc.DialOption) error {
  	var err error
	n.conn, err = grpc.Dial(n.addr, opts...)
	if err != nil {
		return fmt.Errorf("dialing node failed: %v", err)
	}

{{range .Clients}}
	n.{{.}} = New{{.}}(n.conn)
{{end}}

{{range .Services}}
{{if .Multicast}}
  	n.{{.MethodName}}Client, err = n.{{.ServName}}Client.{{.MethodName}}(context.Background())
  	if err != nil {
  		return fmt.Errorf("stream creation failed: %v", err)
  	}
{{end}}
{{end -}}

	return nil
}

func (n *Node) close() error {
	// TODO: Log error, mainly care about the connection error below.
        // We should log this error, but we currently don't have access to the
        // logger in the manager.
{{- range .Services -}}
{{if .Multicast}}
	_, _ = n.{{.MethodName}}Client.CloseAndRecv()
{{- end -}}
{{end}}
	
	if err := n.conn.Close(); err != nil {
                return fmt.Errorf("conn close error: %v", err)
        }	
	return nil
}
`

const qspec_tmpl = `
{{/* Remember to run 'make goldenanddev' after editing this file. */}}

{{- if not .IgnoreImports}}
package {{.PackageName}}
{{- end}}

// QuorumSpec is the interface that wraps every quorum function.
type QuorumSpec interface {
{{- range $elm := .Services}}
{{- if or (.QuorumCall) (.Future)}}
{{- if .QuorumCall}}
	// {{.MethodName}}QF is the quorum function for the {{.MethodName}}
	// quorum call method.
{{- end -}}

{{- if .Future}}
	// {{.MethodName}}QF is the quorum function for the {{.MethodName}}
	// asynchronous quorum call method.
{{- end -}}

{{- if .QFWithReq}}
	{{.MethodName}}QF(req *{{.FQReqName}}, replies []*{{.FQRespName}}) (*{{.FQCustomRespName}}, bool)
{{else}}
	{{.MethodName}}QF(replies []*{{.FQRespName}}) (*{{.FQCustomRespName}}, bool)
{{end}}
{{end -}}

{{- if or (.Correctable) (.CorrectablePrelim)}}
{{if .Correctable}}
	// {{.MethodName}}QF is the quorum function for the {{.MethodName}}
	// correctable quorum call method.
{{- end -}}

{{if .CorrectablePrelim}}
	// {{.MethodName}}QF is the quorum function for the {{.MethodName}} 
	// correctable prelim quourm call method.
{{- end -}}

{{- if .QFWithReq}}
	{{.MethodName}}QF(req *{{.FQReqName}}, replies []*{{.FQRespName}}) (*{{.FQCustomRespName}}, int, bool)
{{else}}
	{{.MethodName}}QF(replies []*{{.FQRespName}}) (*{{.FQCustomRespName}}, int, bool)
{{end}}

{{end -}}
{{- end -}}
}
`

var templates = map[string]string{
	"calltype_common_definitions_tmpl": calltype_common_definitions_tmpl,
	"calltype_correctable_tmpl":        calltype_correctable_tmpl,
	"calltype_correctable_prelim_tmpl": calltype_correctable_prelim_tmpl,
	"calltype_future_tmpl":             calltype_future_tmpl,
	"calltype_multicast_tmpl":          calltype_multicast_tmpl,
	"calltype_quorumcall_tmpl":         calltype_quorumcall_tmpl,
	"node_tmpl":                        node_tmpl,
	"qspec_tmpl":                       qspec_tmpl,
}
