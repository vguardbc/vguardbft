# shellcheck disable=SC2046
kill -9 $(ps aux | grep 'vguard' | awk '{print $2}')
