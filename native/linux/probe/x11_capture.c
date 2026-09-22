// LC-a probe: one X11 root-window image per call, with no image written to disk.
// Build: cc -O2 -Wall -Wextra -o /tmp/daygo-x11-probe x11_capture.c $(pkg-config --cflags --libs x11 xext xrandr)
// Run from an actual Xorg session: /tmp/daygo-x11-probe
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XShm.h>
#include <X11/extensions/Xrandr.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ipc.h>
#include <sys/shm.h>
#include <time.h>

int main(void) {
    const char *session = getenv("XDG_SESSION_TYPE");
    const char *wayland = getenv("WAYLAND_DISPLAY");
    if (session == NULL || strcmp(session, "x11") != 0 ||
        (wayland != NULL && wayland[0] != '\0')) {
        fputs("LC-a requires an actual X11 session; XWayland does not expose the Wayland desktop.\n", stderr);
        return 2;
    }

    Display *display = XOpenDisplay(NULL);
    if (display == NULL) {
        fputs("Cannot open the X11 display.\n", stderr);
        return 2;
    }
    if (!XShmQueryExtension(display)) {
        fputs("XShm is unavailable on this display.\n", stderr);
        XCloseDisplay(display);
        return 2;
    }

    int screen = DefaultScreen(display);
    Window root = RootWindow(display, screen);
    int x = 0, y = 0;
    int width = DisplayWidth(display, screen);
    int height = DisplayHeight(display, screen);
    RROutput primary = XRRGetOutputPrimary(display, root);
    XRRScreenResources *resources = XRRGetScreenResourcesCurrent(display, root);
    if (primary == None || resources == NULL) {
        fputs("No XRandR primary output is configured.\n", stderr);
        if (resources != NULL) XRRFreeScreenResources(resources);
        XCloseDisplay(display);
        return 2;
    }
    XRROutputInfo *output = XRRGetOutputInfo(display, resources, primary);
    if (output == NULL || output->connection != RR_Connected || output->crtc == None) {
        fputs("The XRandR primary output is disconnected.\n", stderr);
        if (output != NULL) XRRFreeOutputInfo(output);
        XRRFreeScreenResources(resources);
        XCloseDisplay(display);
        return 2;
    }
    XRRCrtcInfo *crtc = XRRGetCrtcInfo(display, resources, output->crtc);
    if (crtc == NULL || crtc->width == 0 || crtc->height == 0) {
        fputs("The XRandR primary output has no active geometry.\n", stderr);
        if (crtc != NULL) XRRFreeCrtcInfo(crtc);
        XRRFreeOutputInfo(output);
        XRRFreeScreenResources(resources);
        XCloseDisplay(display);
        return 2;
    }
    x = crtc->x;
    y = crtc->y;
    width = (int)crtc->width;
    height = (int)crtc->height;
    XRRFreeCrtcInfo(crtc);
    XRRFreeOutputInfo(output);
    XRRFreeScreenResources(resources);
    XShmSegmentInfo shm = {.shmid = -1, .shmaddr = NULL, .readOnly = False};
    XImage *image = XShmCreateImage(display, DefaultVisual(display, screen),
                                    DefaultDepth(display, screen), ZPixmap, NULL,
                                    &shm, (unsigned int)width, (unsigned int)height);
    if (image == NULL) {
        fputs("XShmCreateImage failed.\n", stderr);
        XCloseDisplay(display);
        return 2;
    }
    size_t bytes = (size_t)image->bytes_per_line * (size_t)image->height;
    if (image->bytes_per_line <= 0 || bytes == 0 || bytes > 256u * 1024u * 1024u) {
        fputs("Invalid or excessive image size.\n", stderr);
        XDestroyImage(image);
        XCloseDisplay(display);
        return 2;
    }
    shm.shmid = shmget(IPC_PRIVATE, bytes, IPC_CREAT | 0600);
    if (shm.shmid < 0) {
        perror("shmget");
        XDestroyImage(image);
        XCloseDisplay(display);
        return 2;
    }
    shm.shmaddr = shmat(shm.shmid, NULL, 0);
    if (shm.shmaddr == (char *)-1) {
        perror("shmat");
        shmctl(shm.shmid, IPC_RMID, NULL);
        XDestroyImage(image);
        XCloseDisplay(display);
        return 2;
    }
    image->data = shm.shmaddr;
    if (!XShmAttach(display, &shm)) {
        fputs("XShmAttach failed.\n", stderr);
        shmdt(shm.shmaddr);
        image->data = NULL;
        shmctl(shm.shmid, IPC_RMID, NULL);
        XDestroyImage(image);
        XCloseDisplay(display);
        return 2;
    }
    XSync(display, False);
    shmctl(shm.shmid, IPC_RMID, NULL);

    struct timespec start, finish;
    clock_gettime(CLOCK_MONOTONIC, &start);
    int ok = XShmGetImage(display, root, image, x, y, AllPlanes);
    XSync(display, False);
    clock_gettime(CLOCK_MONOTONIC, &finish);
    if (ok) {
        long milliseconds = (finish.tv_sec - start.tv_sec) * 1000L +
                            (finish.tv_nsec - start.tv_nsec) / 1000000L;
        // Only geometry and timing leave the process. Do not print pixels,
        // checksums, window titles or paths from a user's desktop.
        printf("XShmGetImage primary: %dx%d at (%d,%d), %zu bytes, %ld ms\n", width, height, x, y, bytes, milliseconds);
    } else {
        fputs("XShmGetImage failed.\n", stderr);
    }

    XShmDetach(display, &shm);
    XSync(display, False);
    shmdt(shm.shmaddr);
    image->data = NULL;
    XDestroyImage(image);
    XCloseDisplay(display);
    return ok ? 0 : 1;
}
