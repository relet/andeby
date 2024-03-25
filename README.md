# Andeby

A simple macro recorder for android using scrcpy and adb.

## How it works

We use a modified version of scrcpy that records the entire screen whenever it is updated, and calculates a simple "phash" (perceptional hash). 
Please build from https://github.com/relet/scrcpy
This allows the program to identify situations where the screen has (roughly) the same content as previously.

At the same time, we connect to an android device using `adb get-event` to record screen touches. 

Whenever a screen touch is detected, we record

* The screen hash when the gesture has started
* The position of the gesture
* The position where the gesture has ended. 

If the same screen content is detected, that is we calculate the same hash at any point in the future, we replay the gesture. In the case of a tap, an input tap
event is sent. In the case of a swipe, currently a fixed speed swipe is executed to the target point. 

Note that this ignores:
* the speed of the original gesture
* any advanced movements beyond a straight line.