# Third-party notices

The RelayDesk proprietary license applies to original RelayDesk material. It
does not replace licenses for the Go toolchain/runtime, operating-system
components, external utilities, container base images, or test dependencies.

RelayDesk Go code currently imports only the Go standard library. Distributed
Go executables include Go runtime/standard-library code under the notice below.
Linux GUI utilities (xdotool, ImageMagick, xclip and xdg-utils) are installed as
separate OS packages under their respective licenses. Docker bases retain their
own licenses. Python MCP/jsonschema and Node/Playwright are test dependencies;
they are not embedded in the RelayDesk executable. Keep the notices shipped by
those packages when distributing them.

Desktop helpers also use the separately installed Python/PyGObject, GTK,
GStreamer, PipeWire and desktop portal packages on Linux, and Windows
PowerShell/.NET/WinForms on Windows. Their original distribution licenses remain
applicable; RelayDesk does not relicense those runtimes. Inno Setup is an external
Windows installer build tool; observe its distribution/license terms when
building and distributing the resulting setup executable.

## Go runtime and standard library

Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
