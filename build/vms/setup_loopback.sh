function setup_lookback() {
  for i in $(seq 0 7); do
    mknod -m 0660 /dev/loop$i b 7 $i
  done
}

function free_looback() {
  for i in $(seq 0 7); do
    losetup -d /dev/loop$i > /dev/null 2>&1 || true
  done
}

if ! ls -1 /dev/loop? ; then
  setup_lookback
  trap_add free_looback EXIT
fi
