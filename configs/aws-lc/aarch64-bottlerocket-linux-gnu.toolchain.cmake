set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR aarch64)

set(CMAKE_SYSROOT /aarch64-bottlerocket-linux-gnu/sys-root)
set(CMAKE_C_COMPILER aarch64-bottlerocket-linux-gnu-gcc)
set(CMAKE_C_COMPILER_TARGET aarch64-bottlerocket-linux-gnu)
set(CMAKE_CXX_COMPILER aarch64-bottlerocket-linux-gnu-g++)
set(CMAKE_CXX_COMPILER_TARGET aarch64-bottlerocket-linux-gnu)

set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
