#ifdef __cplusplus
extern "C" {
#endif

#include <stdbool.h>
#include <stdint.h>

extern void prompt_callback_bridge(uintptr_t h, char* word);
extern void tokenizer_callback_bridge(uintptr_t h, char* word);

void *llama_allocate_state();

int llama_bootstrap(const char *model_path, void* state_pr, int n_ctx, int n_parts, int memory_type_int);

void* llama_allocate_params(const char *prompt, int seed, int threads, int tokens,
                            int top_k, float top_p, float temp, float repeat_penalty,
                            int repeat_last_n, int n_batch);
void llama_free_params(void* params_ptr);

int llama_predict(void* params_ptr, void* state_pr, uintptr_t cb);

char* llama_print_system_info(void);

void llama_tokenize_prompt(void* state_ptr, const char* prompt, uintptr_t cb);

#ifdef __cplusplus
}
#endif
