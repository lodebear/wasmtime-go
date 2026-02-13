#include <wasm.h>
#include <wasmtime.h>
#include <wasi.h>

wasmtime_store_t *go_store_new(wasm_engine_t *engine, size_t env);
void go_func_new(wasmtime_context_t *context, wasm_functype_t *ty, size_t env, int wrap,  wasmtime_func_t *ret);
wasmtime_error_t *go_linker_define_func(
    wasmtime_linker_t *linker,
    const char *module,
    size_t module_len,
    const char *name,
    size_t name_len,
    const wasm_functype_t *ty,
    int wrap,
    size_t env
);
bool go_externref_new(wasmtime_context_t *cx, size_t env, wasmtime_externref_t *ref);

// Helpers for configuring WASI custom stdout/stderr writers from Go.
void go_wasi_config_set_stdout_custom(wasi_config_t *config, size_t env);
void go_wasi_config_set_stderr_custom(wasi_config_t *config, size_t env);

#define EACH_UNION_ACCESSOR(name) \
  UNION_ACCESSOR(wasmtime_val, i32, int32_t) \
  UNION_ACCESSOR(wasmtime_val, i64, int64_t) \
  UNION_ACCESSOR(wasmtime_val, f32, float) \
  UNION_ACCESSOR(wasmtime_val, f64, double) \
  UNION_ACCESSOR(wasmtime_val, externref, wasmtime_externref_t) \
  UNION_ACCESSOR(wasmtime_val, funcref, wasmtime_func_t) \
  \
  UNION_ACCESSOR(wasmtime_extern, func, wasmtime_func_t) \
  UNION_ACCESSOR(wasmtime_extern, memory, wasmtime_memory_t) \
  UNION_ACCESSOR(wasmtime_extern, sharedmemory, wasmtime_sharedmemory_t*) \
  UNION_ACCESSOR(wasmtime_extern, table, wasmtime_table_t) \
  UNION_ACCESSOR(wasmtime_extern, global, wasmtime_global_t)

// Scalar union accessors for wasmtime_component_val_t.
// These use the same macro pattern but for the component val union.
#define EACH_COMPONENT_VAL_SCALAR_ACCESSOR(name) \
  UNION_ACCESSOR(wasmtime_component_val, boolean, bool) \
  UNION_ACCESSOR(wasmtime_component_val, s8, int8_t) \
  UNION_ACCESSOR(wasmtime_component_val, u8, uint8_t) \
  UNION_ACCESSOR(wasmtime_component_val, s16, int16_t) \
  UNION_ACCESSOR(wasmtime_component_val, u16, uint16_t) \
  UNION_ACCESSOR(wasmtime_component_val, s32, int32_t) \
  UNION_ACCESSOR(wasmtime_component_val, u32, uint32_t) \
  UNION_ACCESSOR(wasmtime_component_val, s64, int64_t) \
  UNION_ACCESSOR(wasmtime_component_val, u64, uint64_t) \
  UNION_ACCESSOR(wasmtime_component_val, f32, float) \
  UNION_ACCESSOR(wasmtime_component_val, f64, double) \
  UNION_ACCESSOR(wasmtime_component_val, character, uint32_t)

#define UNION_ACCESSOR(name, field, ty) \
  ty go_##name##_##field##_get(const name##_t *val); \
  void go_##name##_##field##_set(name##_t *val, ty i);

EACH_UNION_ACCESSOR(UNION_ACCESSOR)
EACH_COMPONENT_VAL_SCALAR_ACCESSOR(UNION_ACCESSOR)

#undef UNION_ACCESSOR

// Complex field accessors for wasmtime_component_val_t.
// These are handwritten because the types don't fit the simple get/set pattern.

// string (wasm_name_t)
wasm_name_t *go_component_val_string_ptr(wasmtime_component_val_t *val);

// list (wasmtime_component_vallist_t)
wasmtime_component_vallist_t *go_component_val_list_ptr(wasmtime_component_val_t *val);

// record (wasmtime_component_valrecord_t)
wasmtime_component_valrecord_t *go_component_val_record_ptr(wasmtime_component_val_t *val);

// tuple (wasmtime_component_valtuple_t)
wasmtime_component_valtuple_t *go_component_val_tuple_ptr(wasmtime_component_val_t *val);

// variant (wasmtime_component_valvariant_t)
wasmtime_component_valvariant_t *go_component_val_variant_ptr(wasmtime_component_val_t *val);

// enum (wasm_name_t)
wasm_name_t *go_component_val_enum_ptr(wasmtime_component_val_t *val);

// option (struct wasmtime_component_val *)
struct wasmtime_component_val *go_component_val_option_get(const wasmtime_component_val_t *val);
void go_component_val_option_set(wasmtime_component_val_t *val, struct wasmtime_component_val *v);

// result (wasmtime_component_valresult_t)
wasmtime_component_valresult_t *go_component_val_result_ptr(wasmtime_component_val_t *val);

// flags (wasmtime_component_valflags_t)
wasmtime_component_valflags_t *go_component_val_flags_ptr(wasmtime_component_val_t *val);

// resource (wasmtime_component_resource_any_t *)
wasmtime_component_resource_any_t *go_component_val_resource_get(const wasmtime_component_val_t *val);
void go_component_val_resource_set(wasmtime_component_val_t *val, wasmtime_component_resource_any_t *r);

// record entry accessors
wasm_name_t *go_component_valrecord_entry_name_ptr(wasmtime_component_valrecord_entry_t *entry);
wasmtime_component_val_t *go_component_valrecord_entry_val_ptr(wasmtime_component_valrecord_entry_t *entry);

// Component item union accessors (for wasmtime_component_item_t)
wasmtime_component_type_t *go_component_item_component_get(const wasmtime_component_item_t *item);
void go_component_item_component_set(wasmtime_component_item_t *item, wasmtime_component_type_t *v);
wasmtime_component_instance_type_t *go_component_item_component_instance_get(const wasmtime_component_item_t *item);
void go_component_item_component_instance_set(wasmtime_component_item_t *item, wasmtime_component_instance_type_t *v);
wasmtime_module_type_t *go_component_item_module_get(const wasmtime_component_item_t *item);
void go_component_item_module_set(wasmtime_component_item_t *item, wasmtime_module_type_t *v);
wasmtime_component_func_type_t *go_component_item_component_func_get(const wasmtime_component_item_t *item);
void go_component_item_component_func_set(wasmtime_component_item_t *item, wasmtime_component_func_type_t *v);
wasmtime_component_resource_type_t *go_component_item_resource_get(const wasmtime_component_item_t *item);
void go_component_item_resource_set(wasmtime_component_item_t *item, wasmtime_component_resource_type_t *v);
wasm_functype_t *go_component_item_core_func_get(const wasmtime_component_item_t *item);
void go_component_item_core_func_set(wasmtime_component_item_t *item, wasm_functype_t *v);
wasmtime_component_valtype_t *go_component_item_type_ptr(wasmtime_component_item_t *item);

// Component valtype union accessors (for wasmtime_component_valtype_t)
wasmtime_component_list_type_t *go_component_valtype_list_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_list_set(wasmtime_component_valtype_t *ty, wasmtime_component_list_type_t *v);
wasmtime_component_record_type_t *go_component_valtype_record_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_record_set(wasmtime_component_valtype_t *ty, wasmtime_component_record_type_t *v);
wasmtime_component_tuple_type_t *go_component_valtype_tuple_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_tuple_set(wasmtime_component_valtype_t *ty, wasmtime_component_tuple_type_t *v);
wasmtime_component_variant_type_t *go_component_valtype_variant_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_variant_set(wasmtime_component_valtype_t *ty, wasmtime_component_variant_type_t *v);
wasmtime_component_enum_type_t *go_component_valtype_enum_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_enum_set(wasmtime_component_valtype_t *ty, wasmtime_component_enum_type_t *v);
wasmtime_component_option_type_t *go_component_valtype_option_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_option_set(wasmtime_component_valtype_t *ty, wasmtime_component_option_type_t *v);
wasmtime_component_result_type_t *go_component_valtype_result_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_result_set(wasmtime_component_valtype_t *ty, wasmtime_component_result_type_t *v);
wasmtime_component_flags_type_t *go_component_valtype_flags_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_flags_set(wasmtime_component_valtype_t *ty, wasmtime_component_flags_type_t *v);
wasmtime_component_resource_type_t *go_component_valtype_own_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_own_set(wasmtime_component_valtype_t *ty, wasmtime_component_resource_type_t *v);
wasmtime_component_resource_type_t *go_component_valtype_borrow_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_borrow_set(wasmtime_component_valtype_t *ty, wasmtime_component_resource_type_t *v);
wasmtime_component_future_type_t *go_component_valtype_future_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_future_set(wasmtime_component_valtype_t *ty, wasmtime_component_future_type_t *v);
wasmtime_component_stream_type_t *go_component_valtype_stream_get(const wasmtime_component_valtype_t *ty);
void go_component_valtype_stream_set(wasmtime_component_valtype_t *ty, wasmtime_component_stream_type_t *v);

// variant field accessors
wasm_name_t *go_component_valvariant_discriminant_ptr(wasmtime_component_valvariant_t *v);
struct wasmtime_component_val *go_component_valvariant_val_get(const wasmtime_component_valvariant_t *v);
void go_component_valvariant_val_set(wasmtime_component_valvariant_t *v, struct wasmtime_component_val *val);

// result field accessors
bool go_component_valresult_is_ok(const wasmtime_component_valresult_t *r);
void go_component_valresult_set_is_ok(wasmtime_component_valresult_t *r, bool is_ok);
struct wasmtime_component_val *go_component_valresult_val_get(const wasmtime_component_valresult_t *r);
void go_component_valresult_val_set(wasmtime_component_valresult_t *r, struct wasmtime_component_val *val);

// Component linker: add_func with Go callback trampoline
wasmtime_error_t *go_component_linker_instance_add_func(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    size_t env
);

// Component linker: add_func_async with Go callback trampoline (async wrapper)
wasmtime_error_t *go_component_linker_instance_add_func_async(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    size_t env
);

// Component async compatibility shims.
wasmtime_call_future_t *go_component_linker_instantiate_async(
    const wasmtime_component_linker_t *linker,
    wasmtime_context_t *context,
    const wasmtime_component_t *component,
    wasmtime_component_instance_t *instance_out,
    wasmtime_error_t **error_ret
);

wasmtime_call_future_t *go_component_func_call_async(
    const wasmtime_component_func_t *func,
    wasmtime_context_t *context,
    const wasmtime_component_val_t *args,
    size_t args_size,
    wasmtime_component_val_t *results,
    size_t results_size,
    wasmtime_error_t **error_ret
);

// Component linker: add_resource with destructor Go callback trampoline
wasmtime_error_t *go_component_linker_instance_add_resource(
    wasmtime_component_linker_instance_t *linker_instance,
    const char *name,
    size_t name_len,
    const wasmtime_component_resource_type_t *resource,
    size_t env
);

// Component linker: optional WASI helpers whose availability differs between
// Wasmtime builds.
wasmtime_error_t *go_component_linker_add_wasip3(
    wasmtime_component_linker_t *linker
);

wasmtime_error_t *go_component_linker_add_wasi_http(
    wasmtime_component_linker_t *linker
);

// Config compatibility helpers for optional/removed C API setters.
void go_config_async_support_set(wasm_config_t *config, bool enabled);
void go_config_wasm_component_model_async_set(wasm_config_t *config, bool enabled);
void go_config_wasm_component_model_error_context_set(wasm_config_t *config, bool enabled);
void go_config_wasm_component_model_async_stackful_set(wasm_config_t *config, bool enabled);
void go_config_wasm_component_model_async_builtins_set(wasm_config_t *config, bool enabled);

void go_wasmtime_context_set_wasi_http(wasmtime_context_t *context);

void go_wasi_config_inherit_network(wasi_config_t *config);
