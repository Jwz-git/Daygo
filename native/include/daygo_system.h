#ifndef DAYGO_SYSTEM_H
#define DAYGO_SYSTEM_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define DG_SYSTEM_ABI_MAJOR 1u
#define DG_SYSTEM_ABI_MINOR 3u
typedef void (*dg_system_event_callback_v1)(uint32_t kind, int64_t at_unix_ns, void *user_data);
enum { DG_SYSTEM_SLEEP=1, DG_SYSTEM_WAKE=2, DG_SYSTEM_SCREEN_LOCKED=3, DG_SYSTEM_SCREEN_UNLOCKED=4, DG_SYSTEM_SCREENSAVER_START=5, DG_SYSTEM_SCREENSAVER_STOP=6, DG_SYSTEM_DISPLAYS_CHANGED=7, DG_SYSTEM_APPLICATION_ACTIVATED=8 };
enum { DG_ACTIVATION_REGULAR=0, DG_ACTIVATION_ACCESSORY=1, DG_ACTIVATION_PROHIBITED=2 };
/* Screen-recording authorization. Preflight cannot tell a fresh not-determined
   state from an explicit denial, so the query never returns DENIED on macOS;
   the value stays in the ABI for callers that can distinguish it. */
enum { DG_PERMISSION_NOT_DETERMINED=0, DG_PERMISSION_GRANTED=1, DG_PERMISSION_DENIED=2 };
enum { DG_SETTINGS_PANE_SCREEN_RECORDING=0, DG_SETTINGS_PANE_NOTIFICATIONS=1, DG_SETTINGS_PANE_LOGIN_ITEMS=2 };
/* Launch-at-login (open the app when the user logs in) via SMAppService.mainApp.
   ENABLED means it will start at next login; REQUIRES_APPROVAL means the user
   turned it off in System Settings > Login Items and must re-approve there;
   NOT_REGISTERED / NOT_FOUND mean off; UNSUPPORTED is returned below macOS 13. */
enum { DG_LAUNCH_AT_LOGIN_NOT_REGISTERED=0, DG_LAUNCH_AT_LOGIN_ENABLED=1, DG_LAUNCH_AT_LOGIN_REQUIRES_APPROVAL=2, DG_LAUNCH_AT_LOGIN_NOT_FOUND=3, DG_LAUNCH_AT_LOGIN_UNSUPPORTED=4 };
int32_t dg_system_start(uint32_t requested_abi_major, dg_system_event_callback_v1 callback, void *user_data);
void dg_system_stop(void);
int32_t dg_activation_policy_set(uint32_t policy);
/* Returns a DG_PERMISSION_* state (>=0), or a negative value on internal error. */
int32_t dg_screen_recording_permission_query(void);
/* Triggers the system prompt on first use; a no-op once denied. Returns
   immediately (0) — the grant only takes effect after the app relaunches, so
   there is no state to read back synchronously. */
int32_t dg_screen_recording_permission_request(void);
/* Opens one DG_SETTINGS_PANE_* pane. Returns 0, or negative on unknown pane. */
int32_t dg_open_system_settings(uint32_t pane);
/* Returns a DG_LAUNCH_AT_LOGIN_* state (>=0), or a negative value on internal
   error. Never prompts. */
int32_t dg_launch_at_login_query(void);
/* Registers (enabled != 0) or unregisters (enabled == 0) the app as a login
   item. Returns 0 on success, negative on failure (e.g. unsigned bundle), and
   DG_LAUNCH_AT_LOGIN_UNSUPPORTED's negative counterpart is not used — below
   macOS 13 it returns a negative error. */
int32_t dg_launch_at_login_set(uint32_t enabled);
#ifdef __cplusplus
}
#endif
#endif
