#!/usr/bin/env python3

import json
import shutil
import subprocess

from pathlib import Path

# -------------------------------------------------------------------
# Base paths
# -------------------------------------------------------------------

BASE_DIR = Path.home() / "UvA" / "Thesis"

GO_CALLSTAT_DIR = BASE_DIR / "go-callstat"
PROJECTS_DIR    = BASE_DIR / "projects_to_analyse"
RESULTS_DIR     = BASE_DIR / "Results"

CONFIG_FILE     = BASE_DIR / "project_config.json"

# -------------------------------------------------------------------
# Load config
# -------------------------------------------------------------------

if CONFIG_FILE.exists():
    with open(CONFIG_FILE, "r", encoding="utf-8") as f:
        PROJECT_CONFIG = json.load(f)
else:
    PROJECT_CONFIG = {}

# -------------------------------------------------------------------
# Setup Results directory and Log file
# -------------------------------------------------------------------

if RESULTS_DIR.exists():
    print(f"[INFO] Clearing results directory: {RESULTS_DIR}")
    shutil.rmtree(RESULTS_DIR)

RESULTS_DIR.mkdir(parents=True, exist_ok=True)

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
# Test cases
# -------------------------------------------------------------------

TEST_CASES = [
    {
        "name": "shallow",
        "depth": 1,
        "no_stdlib": False
    },
    {
        "name": "deep",
        "depth": -1,
        "no_stdlib": False
    },
    {
        "name": "shallow_no_stdlib",
        "depth": 1,
        "no_stdlib": True
    },
    {
        "name": "deep_no_stdlib",
        "depth": -1,
        "no_stdlib": True
    }
]

# -------------------------------------------------------------------
# Analysis Runner
# -------------------------------------------------------------------

def run_analysis(project_path: Path, test_case: dict, log_file):

    project_name = project_path.name

    result_name = test_case["name"]
    depth       = test_case["depth"]
    no_stdlib   = test_case["no_stdlib"]

    print(
        f"\n[INFO] Running {result_name} analysis for {project_name}",
        file=log_file
    )

    # ---------------------------------------------------------------
    # Clean old output
    # ---------------------------------------------------------------

    output_dir = GO_CALLSTAT_DIR / "output"

    if output_dir.exists():
        shutil.rmtree(output_dir)

    # ---------------------------------------------------------------
    # Build command
    # ---------------------------------------------------------------

    relative_project_path = f"../projects_to_analyse/{project_name}"

    cmd = [
        "go", "run", ".",
        f"-dir={relative_project_path}",
        f"-depth={depth}",
        "-no-vis"
    ]

    # Optional no-stdlib flag
    if no_stdlib:
        cmd.append("-no-stdlib")

    # Optional main override
    project_cfg = PROJECT_CONFIG.get(project_name)

    if project_cfg and "main" in project_cfg:
        cmd.append(f"-main={project_cfg['main']}")

    print(f"[CMD] {' '.join(cmd)}", file=log_file)

    # ---------------------------------------------------------------
    # Run process
    # ---------------------------------------------------------------

    result = subprocess.run(
        cmd,
        cwd=GO_CALLSTAT_DIR,
        stdout=log_file,
        stderr=subprocess.STDOUT
    )

    if result.returncode != 0:
        print(
            f"[ERROR] Analysis failed for {project_name}"
            f" (Exit code: {result.returncode})",
            file=log_file
        )
        return

    # ---------------------------------------------------------------
    # Move output
    # ---------------------------------------------------------------

    destination = RESULTS_DIR / project_name / result_name

    destination.parent.mkdir(parents=True, exist_ok=True)

    if output_dir.exists():

        shutil.move(str(output_dir), str(destination))

        print(
            f"[INFO] Moved output to: {destination}",
            file=log_file
        )

    else:
        print(
            f"[WARNING] No output folder found for {project_name}",
            file=log_file
        )

# -------------------------------------------------------------------
# Run all analyses
# -------------------------------------------------------------------

print(f"[INFO] Streaming logs to: {LOG_FILE_PATH}")

with open(LOG_FILE_PATH, "w", encoding="utf-8") as log_f:

    if hasattr(log_f, "reconfigure"):
        log_f.reconfigure(line_buffering=True)

    for project in projects:

        print(
            f"\n==================================================",
            file=log_f
        )

        print(
            f"[PROJECT] {project.name}",
            file=log_f
        )

        print(
            f"==================================================",
            file=log_f
        )

        for test_case in TEST_CASES:
            run_analysis(project, test_case, log_f)

    print("\n[INFO] All analyses complete.", file=log_f)

print("[INFO] Done!")