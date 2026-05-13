package workflow

import "strings"

type EpisodeInput struct {
	Metadata        MetadataState
	Master          ObjectState
	Artifacts       []ObjectState
	ProductionAudio ObjectState
	ProductionKnown bool
	RSSDirty        bool
}

func DecideEpisode(input EpisodeInput, options Options) Decision {
	var decision Decision
	mode := options.normalizedMode()

	addCheck := func(kind, label string, passed bool, reason string) {
		decision.Checks = append(decision.Checks, Check{Kind: kind, Label: label, Passed: passed, Reason: reason})
		if !passed {
			decision.State = StateBlocked
		}
	}
	addOperation := func(operation Operation) {
		if options.autoYes() {
			operation.RequiresPrompt = false
			operation.DefaultYes = true
		}
		decision.Operations = append(decision.Operations, operation)
	}

	metadataApplied := decideMetadata(input.Metadata, addCheck, addOperation)
	masterSynced := decideMaster(input.Master, options, addCheck, addOperation)

	artifactsReady := true
	for _, artifact := range input.Artifacts {
		if !decideArtifact(artifact, addCheck, addOperation) {
			artifactsReady = false
		}
	}

	if mode == ModeJustMaster {
		if decision.State == StateBlocked {
			return decision
		}
		if metadataApplied && masterSynced && artifactsReady {
			decision.State = StateMasterSynced
		} else if metadataApplied {
			decision.State = StateMetadataApplied
		} else {
			decision.State = StateNotStarted
		}
		return decision
	}

	productionSynced := decideProduction(input.ProductionAudio, input.ProductionKnown, masterSynced, options.Reencode, addCheck, addOperation)
	if input.RSSDirty {
		addOperation(Operation{
			Kind:           OperationRegenerateRSS,
			ObjectKind:     ObjectRSS,
			Label:          "podcast RSS",
			Reason:         "episode metadata changed or was repaired",
			RequiresPrompt: true,
		})
	}

	if decision.State == StateBlocked {
		return decision
	}
	switch {
	case input.RSSDirty:
		decision.State = StateRSSReady
	case productionSynced:
		decision.State = StateComplete
	case artifactsReady && masterSynced:
		decision.State = StateArtifactsReady
	case masterSynced:
		decision.State = StateMasterSynced
	case metadataApplied:
		decision.State = StateMetadataApplied
	default:
		decision.State = StateNotStarted
	}
	return decision
}

func decideMetadata(metadata MetadataState, addCheck func(string, string, bool, string), addOperation func(Operation)) bool {
	if metadata.Conflicts {
		addCheck("metadata", "podspec metadata", false, "current metadata conflicts with plan")
		return false
	}
	if metadata.Applied && metadata.Matches {
		addCheck("metadata", "podspec metadata", true, "metadata already applied")
		return true
	}
	if metadata.Planned {
		addCheck("metadata", "podspec metadata", true, "metadata can be applied")
		addOperation(Operation{
			Kind:       OperationWriteMetadata,
			Label:      "podspec metadata",
			Reason:     "planned metadata is not yet present",
			DefaultYes: true,
		})
		return true
	}
	addCheck("metadata", "podspec metadata", false, "no planned metadata")
	return false
}

func decideMaster(master ObjectState, options Options, addCheck func(string, string, bool, string), addOperation func(Operation)) bool {
	label := objectLabel(master, "master")
	if options.Force && !master.Local.Exists {
		addCheck("master", label, false, "forced master sync requires local master")
		return false
	}
	if !master.Local.Exists && !master.Remote.Exists {
		addCheck("master", label, false, "master is missing locally and remotely")
		return false
	}
	if master.Local.Exists && !master.Remote.Exists {
		addCheck("master", label, true, "local master exists and remote is missing")
		addOperation(Operation{
			Kind:           OperationUploadMaster,
			ObjectKind:     ObjectMaster,
			Label:          label,
			Bucket:         master.Bucket,
			Key:            master.Key,
			LocalPath:      master.LocalPath,
			Reason:         "remote master is missing",
			RequiresPrompt: true,
		})
		return true
	}
	if !master.Local.Exists && master.Remote.Exists {
		addCheck("master", label, true, "remote master exists and local is missing")
		addOperation(Operation{
			Kind:       OperationDownloadMaster,
			ObjectKind: ObjectMaster,
			Label:      label,
			Bucket:     master.Bucket,
			Key:        master.Key,
			LocalPath:  master.LocalPath,
			Reason:     "local master is missing",
			DefaultYes: true,
		})
		return true
	}
	if equivalent(master.Local, master.Remote) {
		addCheck("master", label, true, "local and remote master match")
		return true
	}
	addCheck("master", label, true, "local and remote master differ")
	addOperation(Operation{
		Kind:           OperationUploadMaster,
		ObjectKind:     ObjectMaster,
		Label:          label,
		Bucket:         master.Bucket,
		Key:            master.Key,
		LocalPath:      master.LocalPath,
		Reason:         "local and remote master differ",
		RequiresPrompt: true,
	})
	return true
}

func decideArtifact(artifact ObjectState, addCheck func(string, string, bool, string), addOperation func(Operation)) bool {
	label := objectLabel(artifact, "artifact")
	if artifact.Local.Exists && !artifact.Remote.Exists {
		addCheck("artifact", label, true, "local encode-time artifact exists and remote is missing")
		addOperation(Operation{
			Kind:           OperationUploadArtifact,
			ObjectKind:     ObjectEncodeArtifact,
			Label:          label,
			Bucket:         artifact.Bucket,
			Key:            artifact.Key,
			LocalPath:      artifact.LocalPath,
			Reason:         "remote encode-time artifact is missing",
			RequiresPrompt: true,
		})
		return true
	}
	if artifact.Local.Exists && artifact.Remote.Exists {
		if equivalent(artifact.Local, artifact.Remote) {
			addCheck("artifact", label, true, "local and remote encode-time artifact match")
			return true
		}
		addCheck("artifact", label, true, "local and remote encode-time artifact differ")
		addOperation(Operation{
			Kind:           OperationUploadArtifact,
			ObjectKind:     ObjectEncodeArtifact,
			Label:          label,
			Bucket:         artifact.Bucket,
			Key:            artifact.Key,
			LocalPath:      artifact.LocalPath,
			Reason:         "local and remote encode-time artifact differ",
			RequiresPrompt: true,
		})
		return true
	}
	if artifact.Remote.Exists {
		addCheck("artifact", label, true, "remote encode-time artifact exists and local is missing")
		addOperation(Operation{
			Kind:       OperationDownloadArtifact,
			ObjectKind: ObjectEncodeArtifact,
			Label:      label,
			Bucket:     artifact.Bucket,
			Key:        artifact.Key,
			LocalPath:  artifact.LocalPath,
			Reason:     "local encode-time artifact is missing",
			DefaultYes: true,
		})
		return true
	}
	if artifact.Required {
		addCheck("artifact", label, false, "required encode-time artifact is missing locally and remotely")
		return false
	}
	addCheck("artifact", label, true, "optional encode-time artifact is missing")
	return true
}

func decideProduction(production ObjectState, productionKnown, masterSynced, reencode bool, addCheck func(string, string, bool, string), addOperation func(Operation)) bool {
	label := objectLabel(production, "production audio")
	if reencode {
		if !masterSynced {
			addCheck("production", label, false, "production audio re-encode requested but master is not ready")
			return false
		}
		addCheck("production", label, true, "production audio will be regenerated from master")
		addOperation(Operation{
			Kind:       OperationEncode,
			ObjectKind: ObjectProductionAudio,
			Label:      label,
			Reason:     "re-encode requested",
			DefaultYes: true,
		})
		if production.Key != "" {
			reason := "newly encoded production audio will be uploaded"
			destructive := false
			if production.Remote.Exists {
				reason = "remote production audio will be overwritten by re-encoded output"
				destructive = true
			}
			addOperation(Operation{
				Kind:           OperationUploadProduction,
				ObjectKind:     ObjectProductionAudio,
				Label:          label,
				Bucket:         production.Bucket,
				Key:            production.Key,
				LocalPath:      production.LocalPath,
				Reason:         reason,
				RequiresPrompt: true,
				Destructive:    destructive,
			})
		}
		return false
	}
	if productionKnown && production.Remote.Exists && !production.Local.Exists {
		addCheck("production", label, true, "remote production audio exists and metadata is complete")
		return true
	}
	if production.Local.Exists && !production.Remote.Exists {
		addCheck("production", label, true, "local production audio exists and remote is missing")
		addOperation(Operation{
			Kind:           OperationUploadProduction,
			ObjectKind:     ObjectProductionAudio,
			Label:          label,
			Bucket:         production.Bucket,
			Key:            production.Key,
			LocalPath:      production.LocalPath,
			Reason:         "remote production audio is missing",
			RequiresPrompt: true,
		})
		return true
	}
	if production.Local.Exists && production.Remote.Exists {
		if equivalent(production.Local, production.Remote) {
			addCheck("production", label, true, "local and remote production audio match")
			return true
		}
		addCheck("production", label, true, "local and remote production audio differ")
		addOperation(Operation{
			Kind:           OperationUploadProduction,
			ObjectKind:     ObjectProductionAudio,
			Label:          label,
			Bucket:         production.Bucket,
			Key:            production.Key,
			LocalPath:      production.LocalPath,
			Reason:         "local and remote production audio differ",
			RequiresPrompt: true,
		})
		return true
	}
	if production.Remote.Exists {
		addCheck("production", label, true, "remote production audio exists but metadata needs repair")
		addOperation(Operation{
			Kind:       OperationDownloadProduction,
			ObjectKind: ObjectProductionAudio,
			Label:      label,
			Bucket:     production.Bucket,
			Key:        production.Key,
			LocalPath:  production.LocalPath,
			Reason:     "local production audio is needed to repair metadata",
			DefaultYes: true,
		})
		addOperation(Operation{
			Kind:       OperationRepairMetadata,
			ObjectKind: ObjectProductionAudio,
			Label:      label,
			Reason:     "production audio metadata is incomplete",
			DefaultYes: true,
		})
		return true
	}
	if masterSynced {
		addCheck("production", label, true, "production audio is missing and can be encoded from master")
		addOperation(Operation{
			Kind:       OperationEncode,
			ObjectKind: ObjectProductionAudio,
			Label:      label,
			Reason:     "production audio is missing",
			DefaultYes: true,
		})
		return false
	}
	addCheck("production", label, false, "production audio is missing and master is not ready")
	return false
}

func equivalent(local FileState, remote RemoteState) bool {
	if !local.Exists || !remote.Exists {
		return false
	}
	if local.Size != remote.Size {
		return false
	}
	if local.Checksum != "" && remote.Checksum != "" {
		return strings.EqualFold(local.Checksum, remote.Checksum)
	}
	if local.ETag != "" && remote.ETag != "" {
		return strings.EqualFold(strings.Trim(local.ETag, `"`), strings.Trim(remote.ETag, `"`))
	}
	return true
}

func objectLabel(object ObjectState, fallback string) string {
	if strings.TrimSpace(object.Label) != "" {
		return object.Label
	}
	if strings.TrimSpace(object.Key) != "" {
		return object.Key
	}
	return fallback
}
