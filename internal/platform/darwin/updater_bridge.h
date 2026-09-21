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
#endif
