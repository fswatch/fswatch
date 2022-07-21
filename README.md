# WORK IN PROGRESS!!!

I'm still toying around with the optimal interface to meet these goals. Absolutely nothing is stable or complete yet!


# fswatch
filesystem watching / notifications package for Go

# Goals

1. Simple - 
   It should be increadibly easy to use the package correctly.
2. Effective - 
   It should do exactly what is expected, no more and no less.
3. Consistent - 
   This package should make a best effort to act consistently across platforms.

# Non-Goals

1. Similarity to existing APIs -
   There is no desire to match exactly the usage or semantics of inotify, FSEvents,
   ReadDirectoryChangesW, or others.
2. Performance - 
   While good performance is certainly desired, complexity and unreadable source code is not.
3. One-size-fits-all - 
   We hope the package fits the majority of use cases, but many edges cases and rarely used functionality add complexity without bringing obvious benefits to all users.

# License and Acknowledgements

This work would not have been possible without all the valuable information gleaned from the contributors to the `fsnotify/fsnotify` package. 