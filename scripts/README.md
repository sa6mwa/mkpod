# Scripts

This folder contains scripts for use cases coupled with the podcast
[qzj.se](https://qzj.se) or [alltsomkod.se](https://alltsomkod.se). The scripts
may be less useful for your use cases.

## FFmpeg build script

`build-ffmpeg.sh` builds the latest FFmpeg, ffprobe, etc with the latest
`libfdk_aac` library (Fraunhofer FDK AAC). Specifically written for
Ubuntu-based systems, may not work for other Debian-based distributions.

## Export Markers as Chapters in Blender

I use Blender for editing episodes (podcast as well as video). The
`export_markers.py` allows me to use markers in Blender as chapter markers in
podcast episodes which becomes a very simple way to structure chapters. The
script produces a yaml file which can be pasted inside an episode object in
`podcast.yaml`.

How to install the add-on `export_markers.py`:

* In Blender: go to Edit, Preferences, Add-ons
* Click Install.., then choose `export_markers.py`
* Enable the add-on with the checkbox

Use it:

* Go to File, Export, Export Markers as Chapters
* It will save the `chapters.yaml` next to your `.blend` file
* Paste the contents of the yaml under an episode in your `podspec.yaml`
