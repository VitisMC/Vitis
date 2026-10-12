#!/usr/bin/env bash
# Shared lifecycle helpers for the version updater and its rollback tests.

generation_child=""
generation_backup=""
generation_publishing=0
generation_paths=()
generation_present=()
generation_partials=()

# run_generation_command forwards cancellation to the command's complete process group.
run_generation_command() {
  "$@" &
  generation_child=$!
  wait "$generation_child"
  generation_child=""
}

# snapshot_generation_outputs records existing bytes and missing destinations before publication.
snapshot_generation_outputs() {
  generation_backup="$(mktemp -d "$project_root/.generation-backup.XXXXXX")"
  local relative index=0
  for relative in "$@"; do
    if [[ "$relative" == /* || "$relative" == *..* || -L "$project_root/$relative" ]]; then
      echo "unsafe generation output: $relative" >&2
      return 1
    fi
    if [[ -e "$project_root/$relative" ]]; then
      cp -a "$project_root/$relative" "$generation_backup/$index"
      generation_present+=(1)
    else
      generation_present+=(0)
    fi
    generation_paths+=("$relative")
    index=$((index + 1))
  done
}

# cleanup_generation restores snapshots on failure and retains recovery files if restoration fails.
cleanup_generation() {
  local status=$? restore_failed=0 index target
  trap - EXIT INT TERM
  set +e
  if [[ -n "$generation_child" ]]; then
    kill -TERM -- "-$generation_child" 2>/dev/null
    wait "$generation_child" 2>/dev/null
  fi
  if (( status != 0 && generation_publishing == 1 )); then
    echo "generation failed; restoring previous data and generated outputs" >&2
    for ((index=0; index<${#generation_paths[@]}; index++)); do
      target="$project_root/${generation_paths[index]}"
      if ! rm -rf -- "$target"; then
        restore_failed=1
        continue
      fi
      if (( generation_present[index] == 1 )); then
        if ! mkdir -p -- "$(dirname "$target")" || ! cp -a "$generation_backup/$index" "$target"; then
          restore_failed=1
        fi
      fi
    done
  fi
  if (( restore_failed == 1 )); then
    echo "restoration incomplete; recovery snapshot retained at $generation_backup" >&2
    status=1
  elif [[ -n "$generation_backup" ]]; then
    rm -rf -- "$generation_backup" || status=1
  fi
  if [[ -n "${stage_dir:-}" ]]; then rm -rf -- "$stage_dir" || status=1; fi
  if [[ -n "${source_stage:-}" ]]; then rm -rf -- "$source_stage" || status=1; fi
  if [[ -n "${scratch_dir:-}" ]]; then rm -rf -- "$scratch_dir" || status=1; fi
  for target in "${generation_partials[@]}"; do rm -f -- "$target" || status=1; done
  if [[ -n "${generation_lock:-}" ]]; then rmdir -- "$generation_lock" || status=1; fi
  exit "$status"
}

# install_generation_traps enables separate process groups and rollback on both stop signals.
install_generation_traps() {
  set -m
  trap cleanup_generation EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
}
