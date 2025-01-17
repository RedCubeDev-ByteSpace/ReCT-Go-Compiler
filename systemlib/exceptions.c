#include<stdlib.h>
#include<stdio.h>
#include<execinfo.h>
#include<string.h>

#include "objects.h"
#include "exceptions.h"

// Very advanced ReCT exceptions
// -----------------------------

// some ANSI colors for the printout
#define BLK "\e[0;30m"
#define RED "\e[0;31m"
#define GRN "\e[0;32m"
#define YEL "\e[0;33m"
#define BLU "\e[0;34m"
#define MAG "\e[0;35m"
#define CYN "\e[0;36m"
#define WHT "\e[0;37m"

#define BBLK "\e[1;30m"
#define BRED "\e[1;31m"
#define BGRN "\e[1;32m"
#define BYEL "\e[1;33m"
#define BBLU "\e[1;34m"
#define BMAG "\e[1;35m"
#define BCYN "\e[1;36m"
#define BWHT "\e[1;37m"

#define RESET "\e[0m"

// the actual throw message
void exc_Throw(char *message) {
	// exception format:
	// [RUNTIME] Encountered Exception! '<exception>'
	// [Stacktrace]
	// ...

	// error head
	printf("%s[RUNTIME] %sEncountered Exception! %s'%s'\n", BRED, RED, BRED, message);

	// stacktrace
	printf("%s[STACKTRACE] %s\n", BYEL, YEL);

	// get the call stack
	void* callstack[128];
	int frames = backtrace(callstack, 128);
	char** strs = backtrace_symbols(callstack, frames);

	// print out the call stack
	// stop printing as soon as we get to non-program things
	for (int i = 1; i < frames; ++i) {
		// check if this string is from an external lib
		char *foundSO  = strstr(strs[i], ".so");
		char *foundDLL = strstr(strs[i], ".dll");

		// if so, destroy the loop
		if (foundSO)  break;
		if (foundDLL) break;

		printf("%s\n", strs[i]);
	}

	// destroy the strings
	free(strs);

	// die();
	exit(-1);
}

// shortcut for null errors
void exc_ThrowIfNull(void* pointer) {
	if (pointer == NULL)
	 exc_Throw("Null-Pointer exception! The given reference was null.");
}

// shortcut for invalThreadid casting errors
void exc_ThrowIfInvalidCast(class_Any* fromObj, Standard_vTable *to, const char *toFingerprint) {
	// source object hasnt been initialized yet
	// in that case we allow conversion because NULL is the same, no matter what type
	if (fromObj == NULL) return;

	// get the source's vtable for convenience
	Standard_vTable from = fromObj->vtable;

	// goal vTable is null, this is not allowed to happen and indicates a broken program
	if (to == NULL)
		exc_Throw("Conversion vTable for output type could not be found! This indicates a broken executable.");

	// check if the source is the same as the goal already, just casted to something else
	if (strcmp(from.fingerprint, toFingerprint) == 0) return;

	// if the goal type is "Any", all objects are always allowed to cast to it
	if (strcmp(to->className, "Any") == 0) return;

	// check if the source inherits from the goal
	const Standard_vTable *parent = from.parentVTable;
	while (parent != NULL) {
		// inheritance found
		if (strcmp(parent->className, to->className) == 0) return;

		// if not, continue searching up the chain
		parent = parent->parentVTable;
	}

	// check if the goal inherits from the source
	parent = to->parentVTable;
	while (parent != NULL) {
		// inheritance found
		if (strcmp(parent->className, from.className) == 0) return;

		// if not, continue searching up the chain
		parent = parent->parentVTable;
	}

    // if the names are equal -> show the fingerprints
    bool namesAreEqual = strcmp(from.className, to->className) == 0;

	// if all those checks fail -> this cast is invalid
	char *errorTpl = "Object of type %s could not be casted to type %s!";
	char *errorMsg = malloc(snprintf(NULL, 0, errorTpl,
	    namesAreEqual ? from.fingerprint : from.className,
	    namesAreEqual ? toFingerprint    : to->className
	    ) + 1
	);
	sprintf(errorMsg, errorTpl,
	    namesAreEqual ? from.fingerprint : from.className,
    	namesAreEqual ? toFingerprint    : to->className
    );

	exc_Throw(errorMsg);
}