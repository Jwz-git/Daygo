#ifndef DAYGO_UPDATER_BRIDGE_H
#define DAYGO_UPDATER_BRIDGE_H
#include <stdint.h>
int32_t dg_updater_start(void);
void dg_updater_activate(void);
void dg_updater_stop(void);
void dg_updater_check(int32_t interactive);
int32_t dg_updater_automatic(void);
void dg_updater_set_automatic(int32_t enabled);
int32_t dg_updater_checking(void);
int64_t dg_updater_last_checked(void);
/* Localized copy for the install refusal, pushed from Go (docs/05 §5.5.1). An
   empty message leaves the previous copy in place. */
void dg_updater_set_install_refused_message(const char *message);
#endif
