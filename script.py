#!/usr/bin/env python3

import os
import shutil
import subprocess
from pathlib import Path

# Base paths
BASE_DIR = Path.home() / "UvA" / "Thesis"
GO_CALLSTAT_DIR = BASE_DIR / "go-callstat"
PROJECTS_DIR = BASE_DIR / "projects_to_analyse"
RESULTS_DIR = BASE_DIR / "Results"

# -------------------------------------------------------------------
# Setup Results directory and Log file
# -------------------------------------------------------------------

if RESULTS_DIR.exists():
    print(f"[INFO] Clearing results directory: {RESULTS_DIR}")
    shutil.rmtree(RESULTS_DIR)

RESULTS_DIR.mkdir(parents=True, exist_ok=True)

# Define the single log file location
LOG_FILE_PATH = RESULTS_DIR / "analysis.log"

# -------------------------------------------------------------------
# Get all project folders
# -------------------------------------------------------------------

projects = [
    p for p in PROJECTS_DIR.iterdir()
    if p.is_dir()
]

if not projects:
    print("[ERROR] No projects found.")
    exit(1)

# -------------------------------------------------------------------
# Helper function
# -------------------------------------------------------------------

def run_analysis(project_path: Path, depth: int, result_name: str, log_file):
    project_name = project_path.name

    # Using file=log_file redirects Python's prints to your log
    print(f"\n[INFO] Running {result_name} analysis for {project_name}", file=log_file)

    # Remove old output folder if it exists
    output_dir = GO_CALLSTAT_DIR / "output"
    if output_dir.exists():
        shutil.rmtree(output_dir)

    # Relative project path from go-callstat
    relative_project_path = f"../projects_to_analyse/{project_name}"

    # Build command
    cmd = [
        "go", "run", ".",
        f"-dir={relative_project_path}",
        f"-depth={depth}",
        "-no-vis"
    ]

    # Run command and pass the log file handle to stdout/stderr
    result = subprocess.run(
        cmd,
        cwd=GO_CALLSTAT_DIR,
        stdout=log_file,
        stderr=subprocess.STDOUT  # Combines errors and standard output into one stream
    )

    if result.returncode != 0:
        print(f"[ERROR] Analysis failed for {project_name} (Exit code: {result.returncode})", file=log_file)
        return

    # Destination directory
    destination = RESULTS_DIR / project_name / result_name

    # Ensure parent exists
    destination.parent.mkdir(parents=True, exist_ok=True)

    # Move output folder
    if output_dir.exists():
        shutil.move(str(output_dir), str(destination))
        print(f"[INFO] Moved output to: {destination}", file=log_file)
    else:
        print(f"[WARNING] No output folder found for {project_name}", file=log_file)

# -------------------------------------------------------------------
# Run analyses inside a context manager
# -------------------------------------------------------------------

print(f"[INFO] Open log file. Streaming all outputs to: {LOG_FILE_PATH}")

with open(LOG_FILE_PATH, "w", encoding="utf-8") as log_f:
    
    log_f.reconfigure(line_buffering=True) if hasattr(log_f, 'reconfigure') else None

    for project in projects:
        run_analysis(project, 1, "shallow", log_f)
        run_analysis(project, -1, "deep", log_f)

    print("\n[INFO] All analyses complete.", file=log_f)

print("[INFO] Done! Everything has been logged.")