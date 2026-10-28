package script

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPinnedQuickJSLanguageAndPromises(t *testing.T) {
	for _, source := range []string{
		`function main(c) { c.mode='global'; return c }`,
		`const main = (c) => ({...c, mode:'global'})`,
		`async function main(c) { await Promise.resolve(); c.mode='global'; return c }`,
		`function main(c) { return Promise.resolve().then(() => ({...c, mode:'global'})) }`,
		`function main(c) { const names=new Set(['global','global']); c.mode=[...names][0]; return c }`,
		`function main(c) { c.mode=Object.groupBy(['global'],x=>x).global[0]; return c }`,
		`function main(c) { console.log('hello', {a:1}); console.error('test'); c.mode='global'; return c }`,
	} {
		out, err := Evaluate(source, []byte(`{"mode":"rule"}`))
		if err != nil {
			t.Fatal(source, err)
		}
		var value map[string]any
		if err = json.Unmarshal(out, &value); err != nil || value["mode"] != "global" {
			t.Fatal(string(out), err)
		}
	}
	for _, value := range []string{"undefined", "null"} {
		out, err := Evaluate(`function main(c) { c.mode='mutated'; return `+value+` }`, []byte(`{"mode":"rule"}`))
		if err != nil || string(out) != `{"mode":"rule"}` {
			t.Fatal("empty return must retain original profile", string(out), err)
		}
	}
}
func TestQuickJSErrorsAndIsolation(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`const other=1`, "main()"},
		{`function main() { throw new Error('bad profile') }`, "bad profile"},
		{`async function main() { throw new Error('bad promise') }`, "bad promise"},
		{`function main() { return new Promise(()=>{}) }`, "did not settle"},
		{`function main( {`, ":1:"},
		{`function main() { return 1n }`, "BigInt"},
		{`function main() { return ()=>1 }`, "not JSON"},
		{`function main() { const c={}; c.self=c; return c }`, "circular"},
		{`function main() { return new ArrayBuffer(300*1024*1024) }`, "memory"},
	} {
		_, err := Evaluate(test.source, []byte(`{}`))
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
			t.Fatalf("%s: expected %q, got %v", test.source, test.want, err)
		}
	}
	_, err := Evaluate(`function main() { console.log(typeof std,typeof os); return {types:[typeof fetch,typeof require,typeof setTimeout,typeof process]} }`, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Evaluate(`function main() { return [typeof fetch,typeof require,typeof setTimeout,typeof process,typeof std,typeof os] }`, []byte(`{}`))
	if err != nil || string(out) != `["undefined","undefined","undefined","undefined","undefined","undefined"]` {
		t.Fatal(string(out), err)
	}
	_, err = Evaluate(`globalThis.leaked=1; function main(c) { return c }`, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err = Evaluate(`function main() { return {leaked:typeof leaked} }`, []byte(`{}`))
	if err != nil || string(out) != `{"leaked":"undefined"}` {
		t.Fatal("context leaked", string(out), err)
	}
}
func TestQuickJSDeadlineStopsScriptAndMicrotaskLoops(t *testing.T) {
	_, _, err := prepare()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{`function main() { while(true){} }`, `while(true){}`, `async function main() { while(true) await Promise.resolve() }`} {
		start := time.Now()
		_, err = evaluateWithin(source, []byte(`{}`), 100*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "exceeded") || time.Since(start) > 3*time.Second {
			t.Fatalf("deadline failed: %v, %s", err, time.Since(start))
		}
	}
}
func TestQuickJSConcurrentIsolatedContexts(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := Evaluate(`let counter=0; function main() { counter++; return counter }`, []byte(`{}`))
			if err != nil || string(out) != "1" {
				t.Errorf("%s %v", out, err)
			}
		}()
	}
	wg.Wait()
}
