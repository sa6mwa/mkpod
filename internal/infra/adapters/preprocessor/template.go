package preprocessor

var preProcessingTemplate string = `{{ .FFmpeg }} -y -i {{ escape .Input }} -vn -ac 2 -filter_complex "` +
	`pan=stereo|c0<.5*c0+.5*c1|c1<.5*c0+.5*c1,` +

	`{{ if eq .Preset "sm7b" }}` +
	`highpass=80,` +
	`lowpass=18000,` +
	`firequalizer=gain_entry='entry(100,0); entry(200,-6); entry(300,-6); entry(500,-6); entry(600,0); entry(1000,-2); entry(1200,0);entry(7000,0); entry(8000,2); entry(16000,6); entry(20000,0)',` + `compand=attacks=.01:decays=.1:points=-90/-900|-57/-57|-27/-12|-3/-3|0/-3|20/-3:soft-knee=2,` +
	`alimiter=limit=0.7943282347242815:level=disabled` +

	`{{ else if eq .Preset "qzj" }}` +
	`highpass=80,` +
	`lowpass=18000,` +
	`firequalizer=gain_entry='entry(100,0); entry(200,-6); entry(300,-6); entry(500,-6); entry(600,0); entry(1000,-2); entry(1200,0);entry(7000,0); entry(8000,2); entry(16000,6); entry(20000,0)',` + `compand=attacks=.01:decays=.1:points=-90/-900|-57/-57|-27/-9|-3/-3|0/-3|20/-3:soft-knee=2,` +
	`alimiter=limit=0.7943282347242815:level=disabled` +

	`{{ else if eq .Preset "aggressive" }}` +
	`firequalizer=gain_entry='entry(0,-90); entry(50,0); entry(80,0); entry(125,-20); entry(200,0); entry(250,-9); entry(300,-6); entry(1000,0); entry(1400,-3); entry(1700,0); entry(7000,0); entry(10000,+3); entry(13000,+3); entry(16000,+3); entry(18000,-12)',` + `compand=attacks=.01:decays=.1:points=-90/-900|-57/-57|-27/-9|-3/-3|0/-3|20/-3:soft-knee=2,` +
	`firequalizer=gain_entry='entry(80, 0); entry(130,-2); entry(180,0)',` +
	`alimiter=limit=0.7943282347242815:level=disabled` +
	`{{ else if eq .Preset "heavy" }}` +
	`compand=attacks=.01:decays=.1:points=-90/-900|-80/-90|-57/-57|-27/-9|0/-2|20/-2:soft-knee=12,` +
	`alimiter=limit=0.7943282347242815:level=disabled` +
	`{{ else if eq .Preset "qzj-podmic" }}` +
	// `deesser,` +
	`firequalizer=gain_entry='entry(125, +2); entry(250, 0); entry(500, -2); entry(1000, 0); entry(2000, 1); entry(4000, 1); entry(8000, 0); entry(15000, -5)',` +
	`compand=attacks=.01:decays=.1:points=-90/-900|-57/-57|-27/-7|-3/-3|0/-3|20/-3:soft-knee=2,` +
	`alimiter=limit=0.7943282347242815:level=disabled` +

	`{{ else if eq .Preset "qzj-podmic2" }}` +
	// `deesser,` +
	`firequalizer=gain_entry='entry(90,2); entry(538,-3); entry(12000,-2)',` +
	`{{ else if eq .Preset "lowcut" }}` +
	`firequalizer=gain_entry='entry(130,-5); entry(250, 0)',` +
	`compand=attacks=.01:decays=.1:points=-90/-900|-57/-57|-27/-7|-3/-3|0/-3|20/-3:soft-knee=2,` +
	`alimiter=limit=0.7943282347242815:level=disabled` +
	`{{ end }}` +
	`" ` +
	`{{ escape (print .Prefix .Input) }}`
