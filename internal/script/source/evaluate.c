/* Pure configuration transform ABI for the pinned QuickJS-NG engine.
 * Compiled as a WASI reactor; no JS module loader or OS bindings are installed.
 */
#include "quickjs.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static size_t output_size;
static const char *levels[] = {"log", "info", "warn", "error", "debug", "trace"};

static char *output(int failed, const char *value) {
    size_t size = strlen(value);
    char *out = malloc(size + 2);
    if (!out) { output_size = 0; return NULL; }
    out[0] = failed ? '1' : '0';
    memcpy(out + 1, value, size + 1);
    output_size = size + 1;
    return out;
}

static char *describe(JSContext *ctx, JSValue error) {
    JSValue message = JS_UNDEFINED, stack = JS_UNDEFINED;
    if (JS_IsError(error)) {
        message = JS_GetPropertyStr(ctx, error, "message");
        stack = JS_GetPropertyStr(ctx, error, "stack");
    }
    const char *text = JS_ToCString(ctx, JS_IsUndefined(message) ? error : message);
    const char *trace = JS_IsString(stack) ? JS_ToCString(ctx, stack) : NULL;
    size_t size = (text ? strlen(text) : 17) + (trace ? strlen(trace) : 0) + 2;
    char *joined = malloc(size);
    if (joined) snprintf(joined, size, "%s%s%s", text ? text : "JavaScript error", trace && trace[0] ? "\n" : "", trace ? trace : "");
    JS_FreeCString(ctx, text);
    JS_FreeCString(ctx, trace);
    JS_FreeValue(ctx, message);
    JS_FreeValue(ctx, stack);
    return joined;
}

static char *exception(JSContext *ctx) {
    JSValue error = JS_GetException(ctx);
    char *description = describe(ctx, error);
    char *out = output(1, description ? description : "out of memory");
    free(description);
    JS_FreeValue(ctx, error);
    return out;
}

static JSValue console_write(JSContext *ctx, JSValueConst self, int argc, JSValueConst *argv, int magic) {
    (void)self;
    fprintf(stderr, "[script:%s]", levels[magic]);
    for (int i = 0; i < argc; i++) {
        JSValue json = JS_UNDEFINED;
        if (!JS_IsString(argv[i])) json = JS_JSONStringify(ctx, argv[i], JS_UNDEFINED, JS_UNDEFINED);
        const char *text = JS_ToCString(ctx, JS_IsString(argv[i]) ? argv[i] : json);
        fprintf(stderr, " %s", text ? text : "undefined");
        JS_FreeCString(ctx, text);
        JS_FreeValue(ctx, json);
        /* A cyclic/log-only value must not abort the user's transform. */
        if (JS_HasException(ctx)) JS_FreeValue(ctx, JS_GetException(ctx));
    }
    fprintf(stderr, "\n");
    return JS_UNDEFINED;
}

size_t flclash_result_size(void) { return output_size; }

char *flclash_evaluate(const char *script, const char *input) {
    output_size = 0;
    JSRuntime *rt = JS_NewRuntime();
    if (!rt) return output(1, "out of memory");
    JS_SetMemoryLimit(rt, 256 * 1024 * 1024);
    JSContext *ctx = JS_NewContext(rt);
    if (!ctx) { JS_FreeRuntime(rt); return output(1, "out of memory"); }
    char *out = NULL;
    JSValue result = JS_UNDEFINED, entry = JS_UNDEFINED, parsed = JS_UNDEFINED;
    JSValue console = JS_NewObject(ctx);
    for (int i = 0; i < 6; i++) JS_SetPropertyStr(ctx, console, levels[i], JS_NewCFunctionMagic(ctx, console_write, levels[i], 0, JS_CFUNC_generic_magic, i));
    JSValue global = JS_GetGlobalObject(ctx);
    JS_SetPropertyStr(ctx, global, "console", console);
    JS_FreeValue(ctx, global);
    result = JS_Eval(ctx, script, strlen(script), "eval_script", JS_EVAL_TYPE_GLOBAL);
    if (JS_IsException(result)) goto failed;
    JS_FreeValue(ctx, result); result = JS_UNDEFINED;
    entry = JS_Eval(ctx, "main", 4, "eval_script", JS_EVAL_TYPE_GLOBAL);
    if (JS_IsException(entry) || !JS_IsFunction(ctx, entry)) { out = output(1, "script does not define main()"); goto done; }
    parsed = JS_ParseJSON(ctx, input, strlen(input), "profile");
    if (JS_IsException(parsed)) goto failed;
    result = JS_Call(ctx, entry, JS_UNDEFINED, 1, &parsed);
    if (JS_IsException(result)) goto failed;
    JSPromiseStateEnum state = JS_PromiseState(ctx, result);
    if (state != JS_PROMISE_NOT_A_PROMISE) {
        while ((state = JS_PromiseState(ctx, result)) == JS_PROMISE_PENDING) {
            JSContext *job_context = ctx;
            int jobs = JS_ExecutePendingJob(rt, &job_context);
            if (jobs < 0) { out = exception(job_context); goto done; }
            if (jobs == 0) { out = output(1, "main() returned a Promise that did not settle"); goto done; }
        }
        JSValue settled = JS_PromiseResult(ctx, result);
        JS_FreeValue(ctx, result); result = settled;
        if (state == JS_PROMISE_REJECTED) {
            char *description = describe(ctx, result);
            out = output(1, description ? description : "Promise rejected");
            free(description); goto done;
        }
    }
    if (JS_IsUndefined(result) || JS_IsNull(result)) { out = output(0, input); goto done; }
    JSValue encoded = JS_JSONStringify(ctx, result, JS_UNDEFINED, JS_UNDEFINED);
    if (JS_IsException(encoded)) { JS_FreeValue(ctx, encoded); goto failed; }
    if (JS_IsUndefined(encoded)) { out = output(1, "main() returned a value that is not JSON"); JS_FreeValue(ctx, encoded); goto done; }
    const char *text = JS_ToCString(ctx, encoded);
    out = output(text ? 0 : 1, text ? text : "out of memory");
    JS_FreeCString(ctx, text); JS_FreeValue(ctx, encoded);
    goto done;
failed:
    out = exception(ctx);
done:
    JS_FreeValue(ctx, result); JS_FreeValue(ctx, parsed); JS_FreeValue(ctx, entry);
    JS_FreeContext(ctx); JS_FreeRuntime(rt);
    return out;
}
