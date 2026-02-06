#include "_cgo_export.h"
#include "shims.h"

wasmtime_store_t *go_store_new(wasm_engine_t *engine, size_t env) {
  return wasmtime_store_new(engine, (void*) env, goFinalizeStore);
}

static wasm_trap_t* trampoline(
   void *env,
   wasmtime_caller_t *caller,
   const wasmtime_val_t *args,
   size_t nargs,
   wasmtime_val_t *results,
   size_t nresults
) {
    return goTrampolineNew(caller, (size_t) env,
        (wasmtime_val_t*) args, nargs,
        results, nresults);
}

static wasm_trap_t* wrap_trampoline(
   void *env,
   wasmtime_caller_t *caller,
   const wasmtime_val_t *args,
   size_t nargs,
   wasmtime_val_t *results,
   size_t nresults
) {
    return goTrampolineWrap(caller, (size_t) env,
        (wasmtime_val_t*) args, nargs,
        results, nresults);
}

void go_func_new(
    wasmtime_context_t *store,
    wasm_functype_t *ty,
    size_t env,
    int wrap,
    wasmtime_func_t *ret
) {
  wasmtime_func_callback_t callback = trampoline;
  if (wrap)
    callback = wrap_trampoline;
  return wasmtime_func_new(store, ty, callback, (void*) env, NULL, ret);
}

wasmtime_error_t *go_linker_define_func(
    wasmtime_linker_t *linker,
    const char *module,
    size_t module_len,
    const char *name,
    size_t name_len,
    const wasm_functype_t *ty,
    int wrap,
    size_t env
) {
  wasmtime_func_callback_t cb = trampoline;
  void(*finalizer)(void*) = goFinalizeFuncNew;
  if (wrap) {
    cb = wrap_trampoline;
    finalizer = goFinalizeFuncWrap;
  }
  return wasmtime_linker_define_func(linker, module, module_len, name, name_len, ty, cb, (void*) env, finalizer);
}

bool go_externref_new(wasmtime_context_t *cx, size_t env, wasmtime_externref_t *ref) {
  return wasmtime_externref_new(cx, (void*) env, goFinalizeExternref, ref);
}

#define UNION_ACCESSOR(name, field, ty) \
  ty go_##name##_##field##_get(const name##_t *val) { return val->of.field; } \
  void go_##name##_##field##_set(name##_t *val, ty i) { val->of.field = i; }

EACH_UNION_ACCESSOR(UNION_ACCESSOR)
EACH_COMPONENT_VAL_SCALAR_ACCESSOR(UNION_ACCESSOR)

// Complex field accessors for wasmtime_component_val_t

wasm_name_t *go_component_val_string_ptr(wasmtime_component_val_t *val) {
  return &val->of.string;
}

wasmtime_component_vallist_t *go_component_val_list_ptr(wasmtime_component_val_t *val) {
  return &val->of.list;
}

wasmtime_component_valrecord_t *go_component_val_record_ptr(wasmtime_component_val_t *val) {
  return &val->of.record;
}

wasmtime_component_valtuple_t *go_component_val_tuple_ptr(wasmtime_component_val_t *val) {
  return &val->of.tuple;
}

wasmtime_component_valvariant_t *go_component_val_variant_ptr(wasmtime_component_val_t *val) {
  return &val->of.variant;
}

wasm_name_t *go_component_val_enum_ptr(wasmtime_component_val_t *val) {
  return &val->of.enumeration;
}

struct wasmtime_component_val *go_component_val_option_get(const wasmtime_component_val_t *val) {
  return val->of.option;
}

void go_component_val_option_set(wasmtime_component_val_t *val, struct wasmtime_component_val *v) {
  val->of.option = v;
}

wasmtime_component_valresult_t *go_component_val_result_ptr(wasmtime_component_val_t *val) {
  return &val->of.result;
}

wasmtime_component_valflags_t *go_component_val_flags_ptr(wasmtime_component_val_t *val) {
  return &val->of.flags;
}

wasmtime_component_resource_any_t *go_component_val_resource_get(const wasmtime_component_val_t *val) {
  return val->of.resource;
}

void go_component_val_resource_set(wasmtime_component_val_t *val, wasmtime_component_resource_any_t *r) {
  val->of.resource = r;
}

// Record entry accessors

wasm_name_t *go_component_valrecord_entry_name_ptr(wasmtime_component_valrecord_entry_t *entry) {
  return &entry->name;
}

wasmtime_component_val_t *go_component_valrecord_entry_val_ptr(wasmtime_component_valrecord_entry_t *entry) {
  return &entry->val;
}

// Variant field accessors

wasm_name_t *go_component_valvariant_discriminant_ptr(wasmtime_component_valvariant_t *v) {
  return &v->discriminant;
}

struct wasmtime_component_val *go_component_valvariant_val_get(const wasmtime_component_valvariant_t *v) {
  return v->val;
}

void go_component_valvariant_val_set(wasmtime_component_valvariant_t *v, struct wasmtime_component_val *val) {
  v->val = val;
}

// Result field accessors

bool go_component_valresult_is_ok(const wasmtime_component_valresult_t *r) {
  return r->is_ok;
}

void go_component_valresult_set_is_ok(wasmtime_component_valresult_t *r, bool is_ok) {
  r->is_ok = is_ok;
}

struct wasmtime_component_val *go_component_valresult_val_get(const wasmtime_component_valresult_t *r) {
  return r->val;
}

void go_component_valresult_val_set(wasmtime_component_valresult_t *r, struct wasmtime_component_val *val) {
  r->val = val;
}

// Component linker trampolines

static wasmtime_error_t *component_func_trampoline(
    void *env,
    wasmtime_context_t *context,
    const wasmtime_component_func_type_t *ty,
    wasmtime_component_val_t *args,
    size_t nargs,
    wasmtime_component_val_t *results,
    size_t nresults
) {
    return goComponentFuncCallback(
        context, (size_t) env,
        (wasmtime_component_func_type_t *)ty,
        args, nargs,
        results, nresults);
}

wasmtime_error_t *go_component_linker_instance_add_func(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    size_t env
) {
    return wasmtime_component_linker_instance_add_func(
        linker_instance, name, name_len,
        component_func_trampoline, (void*) env,
        goFinalizeComponentFunc);
}

static bool component_func_async_always_ready(void *env) {
    return true;
}

static void component_func_async_trampoline(
    void *env,
    wasmtime_context_t *context,
    const wasmtime_component_func_type_t *ty,
    wasmtime_component_val_t *args, size_t nargs,
    wasmtime_component_val_t *results, size_t nresults,
    wasmtime_error_t **error_ret,
    wasmtime_async_continuation_t *continuation_ret
) {
    wasmtime_error_t *err = goComponentFuncCallback(
        context, (size_t) env,
        (wasmtime_component_func_type_t *)ty,
        args, nargs,
        results, nresults);
    if (err) {
        *error_ret = err;
    }
    continuation_ret->callback = component_func_async_always_ready;
    continuation_ret->env = NULL;
    continuation_ret->finalizer = NULL;
}

wasmtime_error_t *go_component_linker_instance_add_func_async(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    size_t env
) {
    return wasmtime_component_linker_instance_add_func_async(
        linker_instance, name, name_len,
        component_func_async_trampoline, (void*) env,
        goFinalizeComponentFunc);
}

static wasmtime_error_t *component_resource_dtor_trampoline(
    void *env,
    wasmtime_context_t *context,
    uint32_t rep
) {
    return goComponentResourceDtor(context, (size_t) env, rep);
}

wasmtime_error_t *go_component_linker_instance_add_resource(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    const wasmtime_component_resource_type_t *resource,
    size_t env
) {
    return wasmtime_component_linker_instance_add_resource(
        linker_instance, name, name_len,
        resource,
        component_resource_dtor_trampoline, (void*) env,
        goFinalizeComponentResourceDtor);
}

// Component item union accessors

wasmtime_component_type_t *go_component_item_component_get(const wasmtime_component_item_t *item) {
  return item->of.component;
}
void go_component_item_component_set(wasmtime_component_item_t *item, wasmtime_component_type_t *v) {
  item->of.component = v;
}

wasmtime_component_instance_type_t *go_component_item_component_instance_get(const wasmtime_component_item_t *item) {
  return item->of.component_instance;
}
void go_component_item_component_instance_set(wasmtime_component_item_t *item, wasmtime_component_instance_type_t *v) {
  item->of.component_instance = v;
}

wasmtime_module_type_t *go_component_item_module_get(const wasmtime_component_item_t *item) {
  return item->of.module;
}
void go_component_item_module_set(wasmtime_component_item_t *item, wasmtime_module_type_t *v) {
  item->of.module = v;
}

wasmtime_component_func_type_t *go_component_item_component_func_get(const wasmtime_component_item_t *item) {
  return item->of.component_func;
}
void go_component_item_component_func_set(wasmtime_component_item_t *item, wasmtime_component_func_type_t *v) {
  item->of.component_func = v;
}

wasmtime_component_resource_type_t *go_component_item_resource_get(const wasmtime_component_item_t *item) {
  return item->of.resource;
}
void go_component_item_resource_set(wasmtime_component_item_t *item, wasmtime_component_resource_type_t *v) {
  item->of.resource = v;
}

wasm_functype_t *go_component_item_core_func_get(const wasmtime_component_item_t *item) {
  return item->of.core_func;
}
void go_component_item_core_func_set(wasmtime_component_item_t *item, wasm_functype_t *v) {
  item->of.core_func = v;
}

wasmtime_component_valtype_t *go_component_item_type_ptr(wasmtime_component_item_t *item) {
  return &item->of.type;
}

// Component valtype union accessors

wasmtime_component_list_type_t *go_component_valtype_list_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.list;
}
void go_component_valtype_list_set(wasmtime_component_valtype_t *ty, wasmtime_component_list_type_t *v) {
  ty->of.list = v;
}

wasmtime_component_record_type_t *go_component_valtype_record_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.record;
}
void go_component_valtype_record_set(wasmtime_component_valtype_t *ty, wasmtime_component_record_type_t *v) {
  ty->of.record = v;
}

wasmtime_component_tuple_type_t *go_component_valtype_tuple_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.tuple;
}
void go_component_valtype_tuple_set(wasmtime_component_valtype_t *ty, wasmtime_component_tuple_type_t *v) {
  ty->of.tuple = v;
}

wasmtime_component_variant_type_t *go_component_valtype_variant_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.variant;
}
void go_component_valtype_variant_set(wasmtime_component_valtype_t *ty, wasmtime_component_variant_type_t *v) {
  ty->of.variant = v;
}

wasmtime_component_enum_type_t *go_component_valtype_enum_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.enum_;
}
void go_component_valtype_enum_set(wasmtime_component_valtype_t *ty, wasmtime_component_enum_type_t *v) {
  ty->of.enum_ = v;
}

wasmtime_component_option_type_t *go_component_valtype_option_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.option;
}
void go_component_valtype_option_set(wasmtime_component_valtype_t *ty, wasmtime_component_option_type_t *v) {
  ty->of.option = v;
}

wasmtime_component_result_type_t *go_component_valtype_result_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.result;
}
void go_component_valtype_result_set(wasmtime_component_valtype_t *ty, wasmtime_component_result_type_t *v) {
  ty->of.result = v;
}

wasmtime_component_flags_type_t *go_component_valtype_flags_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.flags;
}
void go_component_valtype_flags_set(wasmtime_component_valtype_t *ty, wasmtime_component_flags_type_t *v) {
  ty->of.flags = v;
}

wasmtime_component_resource_type_t *go_component_valtype_own_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.own;
}
void go_component_valtype_own_set(wasmtime_component_valtype_t *ty, wasmtime_component_resource_type_t *v) {
  ty->of.own = v;
}

wasmtime_component_resource_type_t *go_component_valtype_borrow_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.borrow;
}
void go_component_valtype_borrow_set(wasmtime_component_valtype_t *ty, wasmtime_component_resource_type_t *v) {
  ty->of.borrow = v;
}

wasmtime_component_future_type_t *go_component_valtype_future_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.future;
}
void go_component_valtype_future_set(wasmtime_component_valtype_t *ty, wasmtime_component_future_type_t *v) {
  ty->of.future = v;
}

wasmtime_component_stream_type_t *go_component_valtype_stream_get(const wasmtime_component_valtype_t *ty) {
  return ty->of.stream;
}
void go_component_valtype_stream_set(wasmtime_component_valtype_t *ty, wasmtime_component_stream_type_t *v) {
  ty->of.stream = v;
}
