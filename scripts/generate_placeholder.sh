#!/bin/bash
# Script to generate a premium professional television broadcast station network screen for Plexishow.
# Imports bg_broadcast.png as scaled background, adds glowing pulsing indicators.
# Completely silent video with no audio track to keep things extremely clean.
# Perfectly matches IPTV stream parameters: 1920x1080 Full HD.

set -e

mkdir -p assets

# Detect available encoder
if ffmpeg -encoders 2>/dev/null | grep -q libx264; then
  ENCODER="libx264"
  ENC_OPTS="-pix_fmt yuv420p -profile:v main -level 4.1 -preset ultrafast"
else
  echo "WARNING: libx264 encoder not found in FFmpeg. Falling back to native mpeg2video."
  ENCODER="mpeg2video"
  ENC_OPTS="-pix_fmt yuv420p -b:v 2M"
fi

# Escape colons inside drawtext options for FFmpeg filter graph parsing.
echo "Generating 30-second premium television broadcast station placeholder video using $ENCODER..."
ffmpeg -y \
  -loop 1 -r 25 -i assets/bg_broadcast.png \
  -filter_complex "[0:v]scale=1920:1080,drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='● TUNING CHANNEL':fontcolor=0xFF3333:fontsize=48:x=100:y=920:alpha='(1+sin(2*PI*t))/2':enable='lt(t,25)',drawbox=x=0:y=0:w=1920:h=1080:color=black@0.85:t=fill:enable='gte(t,25)',drawbox=x=360:y=370:w=1200:h=340:color=0xFF3333@0.2:t=fill:enable='gte(t,25)',drawbox=x=360:y=370:w=1200:h=340:color=0xFF3333:t=5:enable='gte(t,25)',drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='[ ERROR\: SIGNAL EXCURSION LIMIT EXCEEDED ]':fontcolor=0xFF3333:fontsize=40:x=400:y=430:enable='gte(t,25)',drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf:text='No response received from the local tuner.':fontcolor=0xDDDDDD:fontsize=32:x=400:y=510:enable='gte(t,25)',drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf:text='Closing client connection to release resources...':fontcolor=0x999999:fontsize=28:x=400:y=590:enable='gte(t,25)'[v]" \
  -map "[v]" \
  -c:v $ENCODER $ENC_OPTS \
  -an \
  -t 30 \
  -f mpegts assets/placeholder.ts

echo "Success! assets/placeholder.ts created successfully."
