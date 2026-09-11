#ifndef DAYGO_SYSTEM_H
#define DAYGO_SYSTEM_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define DG_SYSTEM_ABI_MAJOR 1u
#define DG_SYSTEM_ABI_MINOR 0u
typedef void (*dg_system_event_callback_v1)(uint32_t kind, int64_t at_unix_ns, void *user_data);
enum { DG_SYSTEM_SLEEP=1, DG_SYSTEM_WAKE=2, DG_SYSTEM_SCREEN_LOCKED=3, DG_SYSTEM_SCREEN_UNLOCKED=4, DG_SYSTEM_SCREENSAVER_START=5, DG_SYSTEM_SCREENSAVER_STOP=6, DG_SYSTEM_DISPLAYS_CHANGED=7 };
int32_t dg_system_start(uint32_t requested_abi_major, dg_system_event_callback_v1 callback, void *user_data);
void dg_system_stop(void);
#ifdef __cplusplus
}
#endif
#endif
