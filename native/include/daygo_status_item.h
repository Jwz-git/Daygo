#ifndef DAYGO_STATUS_ITEM_H
#define DAYGO_STATUS_ITEM_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define DG_STATUS_ITEM_ABI_MAJOR 2u
typedef void (*dg_status_item_action_callback_v1)(uint32_t action, void *user_data);
enum {
  DG_STATUS_ITEM_OPEN = 1,
  /* The single primary action shown when the recorder is not capturing: start
     when idle, resume when paused. The app layer resolves which one from the
     recorder state, so the adapter emits one code for both. */
  DG_STATUS_ITEM_TOGGLE_PAUSE = 2,
  DG_STATUS_ITEM_QUIT = 3,
  DG_STATUS_ITEM_OPEN_RECORDINGS = 4,
  /* Pause-duration submenu items, shown only while capturing. */
  DG_STATUS_ITEM_PAUSE_INDEFINITE = 6,
  DG_STATUS_ITEM_PAUSE_15 = 7,
  DG_STATUS_ITEM_PAUSE_30 = 8,
  DG_STATUS_ITEM_PAUSE_60 = 9
};
typedef struct dg_status_item_state_v1 {
  uint32_t visible;
  /* When set, render pause_menu_label as a submenu of the four pause_* items.
     When clear, render primary_action_label as a single item. */
  uint32_t pause_durations_enabled;
  /* Enables the single primary action (disabled while starting). */
  uint32_t primary_action_enabled;
  const char *title;
  const char *tooltip;
  const char *open_label;
  const char *recordings_label;
  const char *quit_label;
  const char *pause_menu_label;
  const char *pause_15_label;
  const char *pause_30_label;
  const char *pause_60_label;
  const char *pause_indefinite_label;
  const char *primary_action_label;
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
