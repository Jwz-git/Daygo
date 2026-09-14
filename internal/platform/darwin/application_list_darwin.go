//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// One packed record stream, parsed by decodeApplicationList in Go:
//   uint64_t count (little-endian)
//   count times: uint64_t identifier_len, uint64_t name_len,
//                identifier bytes, name bytes (both UTF-8)
//
// The enumeration mirrors what the capture privacy filter can act on: NSBundle
// resolves the same identifiers NSWorkspace uses for lookups, and a bundle
// without one is skipped fail-closed rather than listed. Directory contents
// are sorted before deduplication so repeated calls agree on which copy of a
// duplicate identifier survives — the packed length must stay stable across
// calls, because a caller sizes its buffer from a first pass.
//
// The returned buffer is malloc'd and owned by the caller.
// Resolves the display name of a bundle in one requested language. Daygo's UI
// language is a product setting, not the process's AppleLanguages, so the
// localized Info.plist strings are looked up explicitly instead of letting
// Foundation pick by process preference. Candidates are BCP-47 tag variants
// because bundles name their localizations inconsistently (zh_CN vs zh-Hans).
//
// Lookup order: the modern InfoPlist.loctable (a plain binary plist keyed by
// locale), then the legacy per-lproj InfoPlist.strings, then nil — the caller
// falls back to the unlocalized name chain.
static NSString *dg_localized_display_name(NSBundle *bundle, NSString *language) {
    if (language.length == 0) return nil;

    NSMutableArray<NSString *> *keys = [NSMutableArray array];
    NSString *underscored = [language stringByReplacingOccurrencesOfString:@"-" withString:@"_"];
    [keys addObject:underscored];
    [keys addObject:language];
    [keys addObject:[underscored componentsSeparatedByString:@"_"].firstObject];
    if ([underscored hasPrefix:@"zh"]) {
        [keys addObject:@"zh_Hans"];
        [keys addObject:@"zh-Hans"];
    }

    NSString *loctablePath = [bundle pathForResource:@"InfoPlist" ofType:@"loctable"];
    if (loctablePath) {
        NSDictionary *table = [NSDictionary dictionaryWithContentsOfFile:loctablePath];
        for (NSString *key in keys) {
            NSDictionary *entry = table[key];
            if (![entry isKindOfClass:[NSDictionary class]]) continue;
            NSString *name = entry[@"CFBundleDisplayName"] ?: entry[@"CFBundleName"];
            if ([name isKindOfClass:[NSString class]] && name.length > 0) return name;
        }
    }
    for (NSString *key in keys) {
        NSString *stringsPath = [bundle pathForResource:@"InfoPlist" ofType:@"strings"
                                           inDirectory:nil forLocalization:key];
        if (stringsPath == nil) continue;
        NSDictionary *strings = [NSDictionary dictionaryWithContentsOfFile:stringsPath];
        NSString *name = strings[@"CFBundleDisplayName"] ?: strings[@"CFBundleName"];
        if ([name isKindOfClass:[NSString class]] && name.length > 0) return name;
    }
    return nil;
}

static void dg_list_add_bundle(NSString *path, NSString *language,
                               NSMutableSet<NSString *> *seen,
                               NSMutableArray<NSString *> *ids,
                               NSMutableArray<NSString *> *names) {
    NSBundle *bundle = [NSBundle bundleWithPath:path];
    NSString *identifier = bundle.bundleIdentifier;
    if (identifier.length == 0 || [identifier lengthOfBytesUsingEncoding:NSUTF8StringEncoding] > 4096) return;
    if ([seen containsObject:identifier]) return;

    NSString *localized = dg_localized_display_name(bundle, language);
    id display = [bundle objectForInfoDictionaryKey:@"CFBundleDisplayName"];
    id bundleName = [bundle objectForInfoDictionaryKey:@"CFBundleName"];
    NSCharacterSet *whitespace = [NSCharacterSet whitespaceAndNewlineCharacterSet];
    NSString *fallback = [[path lastPathComponent] stringByDeletingPathExtension];
    // Array literals crash on a nil element, and both dictionary keys are
    // optional — so candidates are collected explicitly instead.
    NSMutableArray<NSString *> *candidates = [NSMutableArray array];
    if (localized.length > 0) [candidates addObject:localized];
    if ([display isKindOfClass:[NSString class]]) [candidates addObject:display];
    if ([bundleName isKindOfClass:[NSString class]]) [candidates addObject:bundleName];
    [candidates addObject:fallback];
    NSString *name = nil;
    for (NSString *candidate in candidates) {
        NSString *trimmed = [candidate stringByTrimmingCharactersInSet:whitespace];
        if (trimmed.length > 0) { name = trimmed; break; }
    }
    if (name.length == 0 || [name lengthOfBytesUsingEncoding:NSUTF8StringEncoding] > 4096) return;

    [seen addObject:identifier];
    [ids addObject:identifier];
    [names addObject:name];
}

static void dg_list_append_u64(NSMutableData *data, uint64_t value) {
    uint64_t little = OSSwapHostToLittleInt64(value);
    [data appendBytes:&little length:8];
}

// Returns malloc'd packed bytes with *out_len set, or NULL with *out_len == 0
// when nothing was enumerated. An empty result is a valid answer, not an
// error: the grid simply has nothing to show. language is Daygo's UI language
// as a BCP-47 tag ("zh-CN", "en"); an empty string keeps the unlocalized
// fallback chain.
static uint8_t *dg_ls_installed_apps(const char *language_bytes, int64_t *out_len) {
    NSString *language = language_bytes == NULL
        ? @""
        : [NSString stringWithUTF8String:language_bytes];
    if (out_len == NULL) return NULL;
    *out_len = 0;

    NSArray<NSString *> *roots = @[
        @"/Applications",
        @"/System/Applications",
        [NSHomeDirectory() stringByAppendingPathComponent:@"Applications"],
    ];

    NSMutableSet<NSString *> *seen = [NSMutableSet set];
    NSMutableArray<NSString *> *ids = [NSMutableArray array];
    NSMutableArray<NSString *> *names = [NSMutableArray array];

    for (NSString *root in roots) {
        NSArray<NSString *> *children = [[NSFileManager defaultManager]
            contentsOfDirectoryAtPath:root error:nil];
        if (children == nil) continue;
        children = [children sortedArrayUsingSelector:@selector(compare:)];

        // Depth is capped at two: the root's own .app bundles plus one folder
        // level such as /Applications/Utilities. Deeper bundles belong to
        // other applications (simulator runtimes and the like) and must not
        // pose as the user's applications.
        for (NSString *child in children) {
            if (ids.count >= 4096) goto done;
            NSString *childPath = [root stringByAppendingPathComponent:child];
            if ([child.lowercaseString hasSuffix:@".app"]) {
                dg_list_add_bundle(childPath, language, seen, ids, names);
                continue;
            }
            BOOL isDirectory = NO;
            NSFileManager *fileManager = [NSFileManager defaultManager];
            if (![fileManager fileExistsAtPath:childPath isDirectory:&isDirectory] || !isDirectory) {
                continue;
            }
            NSArray<NSString *> *inner = [fileManager contentsOfDirectoryAtPath:childPath error:nil];
            inner = [inner sortedArrayUsingSelector:@selector(compare:)];
            for (NSString *item in inner) {
                if (ids.count >= 4096) goto done;
                if (![item.lowercaseString hasSuffix:@".app"]) continue;
                dg_list_add_bundle([childPath stringByAppendingPathComponent:item],
                                   language, seen, ids, names);
            }
        }
    }

done: {
    NSMutableData *payload = [NSMutableData dataWithCapacity:8 + ids.count * 24];
    dg_list_append_u64(payload, (uint64_t)ids.count);
    for (NSUInteger i = 0; i < ids.count; i++) {
        NSData *identifierBytes = [(NSString *)ids[i] dataUsingEncoding:NSUTF8StringEncoding];
        NSData *nameBytes = [(NSString *)names[i] dataUsingEncoding:NSUTF8StringEncoding];
        dg_list_append_u64(payload, (uint64_t)identifierBytes.length);
        dg_list_append_u64(payload, (uint64_t)nameBytes.length);
        [payload appendData:identifierBytes];
        [payload appendData:nameBytes];
    }
    uint8_t *out = malloc(payload.length ? payload.length : 1);
    if (out == NULL) return NULL;
    memcpy(out, payload.bytes, payload.length);
    *out_len = (int64_t)payload.length;
    return out;
}
}
*/
import "C"

import (
	"context"
	"encoding/binary"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// maximumApplicationListBytes bounds the packed payload. At roughly 50 bytes
// per application this admits thousands of entries; a larger result would mean
// something went wrong rather than that the user owns that many apps.
const maximumApplicationListBytes = 4 << 20

// listApplications enumerates the installed applications through Foundation.
// See the Objective-C block above for the wire format and the enumeration
// rules. The buffer the native side mallocs is copied into Go and freed before
// return; nothing native survives the call.
func listApplications(ctx context.Context, language string) ([]platform.AppInfo, error) {
	var languageBytes *C.char
	if language != "" {
		languageBytes = C.CString(language)
		defer C.free(unsafe.Pointer(languageBytes))
	}
	var length C.int64_t
	data := C.dg_ls_installed_apps(languageBytes, &length)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if data == nil {
		return []platform.AppInfo{}, nil
	}
	defer C.free(unsafe.Pointer(data))
	if length <= 0 || uint64(length) > maximumApplicationListBytes {
		return nil, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	records := C.GoBytes(unsafe.Pointer(data), C.int(length))
	return decodeApplicationList(records)
}

// decodeApplicationList parses the packed stream. Every length is checked
// against the remaining bytes before slicing, so a truncated or corrupt
// payload fails closed instead of panicking or inventing entries.
func decodeApplicationList(records []byte) ([]platform.AppInfo, error) {
	if len(records) < 8 {
		return nil, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	count := binary.LittleEndian.Uint64(records)
	records = records[8:]

	applications := make([]platform.AppInfo, 0, min(uint64(count), uint64(len(records)/4)))
	for index := uint64(0); index < count; index++ {
		if len(records) < 16 {
			return nil, &platform.ApplicationError{Code: platform.ApplicationNative}
		}
		idLength := int(binary.LittleEndian.Uint64(records))
		nameLength := int(binary.LittleEndian.Uint64(records[8:]))
		records = records[16:]
		if idLength <= 0 || nameLength <= 0 || idLength+nameLength > len(records) {
			return nil, &platform.ApplicationError{Code: platform.ApplicationNative}
		}
		identifier := string(records[:idLength])
		name := string(records[idLength : idLength+nameLength])
		records = records[idLength+nameLength:]
		applications = append(applications, platform.AppInfo{ID: identifier, Name: name})
	}
	return applications, nil
}
