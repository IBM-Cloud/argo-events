#!/bin/bash

echo "Starting Knative webhook-gateway process"

trap 'gracefull_exit' 15

gracefull_exit()
{
  echo "Caught SIGTERM..."
  childPids=`ps -o pid --no-headers --ppid $$`
  for childPid in $childPids
  do
      echo "Killing child background process $childPid"
      kill -9 $childPid 2>/dev/null
  done

  exit 1
}

# Start the first process
bin/webhook-gateway &
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start my_first_process: $status"
  exit $status
fi

sleep 1

# Start the second process
echo "Starting gateway-client process"
bin/gateway-client &
status=$?
if [ $status -ne 0 ]; then
  echo "Failed to start my_second_process: $status"
  exit $status
fi

# Naive check runs checks once a minute to see if either of the processes exited.
# This illustrates part of the heavy lifting you need to do if you want to run
# more than one service in a container. The container exits with an error
# if it detects that either of the processes has exited.
# Otherwise it loops forever, waking up every 60 seconds
echo "Monitoring"
while sleep 60; do
  ps aux |grep webhook-gateway |grep -q -v grep
  PROCESS_1_STATUS=$?
  ps aux |grep gateway-client |grep -q -v grep
  PROCESS_2_STATUS=$?
  # If the greps above find anything, they exit with 0 status
  # If they are not both 0, then something is wrong
  if [ $PROCESS_1_STATUS -ne 0 -o $PROCESS_2_STATUS -ne 0 ]; then
    echo "One of the processes has already exited."
    exit 1
  fi
done
