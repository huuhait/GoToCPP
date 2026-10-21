#!/usr/bin/env python3
"""Log the actual compiler argv without changing it; used only by prepare.sh."""

import json
import os
import subprocess
import sys

command = sys.argv[1:]
log = os.environ["MODULE_CAPTURE"]
record = json.dumps({"cwd": os.getcwd(), "argv": command}) + "\n"
descriptor = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
try:
    os.write(descriptor, record.encode())
finally:
    os.close(descriptor)
sys.exit(subprocess.run(command).returncode)
