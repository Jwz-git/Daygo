#ifndef DAYGO_STATUS_ITEM_H
#define DAYGO_STATUS_ITEM_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define DG_STATUS_ITEM_ABI_MAJOR 1u
typedef void (*dg_status_item_action_callback_v1)(uint32_t action, void *user_data);
enum { DG_STATUS_ITEM_OPEN = 1, DG_STATUS_ITEM_TOGGLE_PAUSE = 2, DG_STATUS_ITEM_QUIT = 3 };
typedef struct dg_status_item_state_v1 {
  uint32_t visible;
  uint32_t pause_enabled;
  const char *title;
  const char *tooltip;
  const char *open_label;
  const char *pause_label;
  const char *quit_label;
} dg_status_item_state_v1;
int32_t dg_status_item_set(uint32_t requested_abi_major,
                           const dg_status_item_state_v1 *state,
                           dg_status_item_action_callback_v1 callback,
                           void *user_data);
void dg_status_item_stop(void);
#ifdef __cplusplus
}
#endif
#endif
