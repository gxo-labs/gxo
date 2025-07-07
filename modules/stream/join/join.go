package join

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gxo-labs/gxo/internal/logger"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/paramutil"
	intTemplate "github.com/gxo-labs/gxo/internal/template"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

func init() {
	module.Register("stream:join", NewJoinModule)
}

type JoinModule struct{}

func NewJoinModule() plugin.Module { return &JoinModule{} }

type buildEntry struct {
	record  map[string]interface{}
	matched bool
}

type joinKeyConfig struct {
	StreamName   string
	Field        string
	Template     string
	IsProbe      bool
	inputChannel <-chan map[string]interface{}
}

type buildRecord struct {
	StreamName string
	Record     map[string]interface{}
	Key        interface{}
}

type moduleConfig struct {
	joinType        string
	buildStreams    []joinKeyConfig
	probeStream     joinKeyConfig
	mergeStrategy   string
	nestingKeys     map[string]string
	maxBuildRecords int64
}

type buildStats struct {
	totalBuildRecords int64
	recordsSkipped    int64
}

type probeStats struct {
	probeRecordsProcessed int64
	recordsJoined         int64
	recordsSkipped        int64
}

type postProbeStats struct {
	recordsJoined int64
}

func (m *JoinModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader state.StateReader,
	inputs map[string]<-chan map[string]interface{},
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	log := logger.NewDefaultLogger("debug").With("module", "stream:join")

	cfg, err := parseAndValidateParams(params, inputs)
	if err != nil {
		log.Errorf("Parameter validation failed: %v", err)
		return nil, err
	}
	log.Debugf("Join config validated. Type: %s, Strategy: %s", cfg.joinType, cfg.mergeStrategy)

	// Create a temporary renderer instance for key extraction. We pass nil for the
	// eventBus and secretTracker as they are not needed for this module's templates.
	renderer := intTemplate.NewGoRenderer(nil, nil, nil)

	log.Debugf("--- Starting Build Phase ---")
	buildMap, bStats, buildErr := m.runBuildPhase(ctx, cfg, renderer, stateReader, errChan, log)
	if buildErr != nil {
		return nil, buildErr
	}
	log.Debugf("--- Build Phase Complete. Unique Keys: %d, Total Records: %d ---", len(buildMap), bStats.totalBuildRecords)

	log.Debugf("--- Starting Probe Phase ---")
	pStats, probeErr := m.runProbePhase(ctx, cfg, renderer, stateReader, buildMap, outputChans, errChan, log)
	if probeErr != nil {
		return nil, probeErr
	}
	log.Debugf("--- Probe Phase Complete. Processed: %d, Joined: %d ---", pStats.probeRecordsProcessed, pStats.recordsJoined)

	log.Debugf("--- Starting Post-Probe Phase ---")
	ppStats, postProbeErr := m.runPostProbePhase(ctx, cfg, buildMap, outputChans)
	if postProbeErr != nil {
		log.Errorf("Error during post-probe phase: %v", postProbeErr)
		return nil, postProbeErr
	}
	log.Debugf("--- Post-Probe Phase Complete. Joined: %d ---", ppStats.recordsJoined)

	finalSummary := map[string]interface{}{
		"records_joined":       pStats.recordsJoined + ppStats.recordsJoined,
		"probe_stream_records": pStats.probeRecordsProcessed,
		"build_stream_records": bStats.totalBuildRecords,
		"skipped_key_errors":   bStats.recordsSkipped + pStats.recordsSkipped,
	}
	log.Infof("Join complete. Total records joined: %d", finalSummary["records_joined"])
	return finalSummary, nil
}

func (m *JoinModule) runBuildPhase(ctx context.Context, cfg *moduleConfig, renderer intTemplate.Renderer, state state.StateReader, errChan chan<- error, log gxolog.Logger) (map[interface{}]map[string][]*buildEntry, *buildStats, error) {
	buildMap := make(map[interface{}]map[string][]*buildEntry)
	stats := &buildStats{}
	var buildErr error
	var buildErrOnce sync.Once
	buildCtx, cancelBuild := context.WithCancel(ctx)
	defer cancelBuild()

	mergedBuildChan := make(chan buildRecord, 100*len(cfg.buildStreams))
	var buildWg sync.WaitGroup
	buildWg.Add(len(cfg.buildStreams))

	for _, streamConfig := range cfg.buildStreams {
		go func(sCfg joinKeyConfig) {
			defer buildWg.Done()
			for record := range sCfg.inputChannel {
				if buildCtx.Err() != nil || buildErr != nil {
					continue
				}

				currentCount := atomic.AddInt64(&stats.totalBuildRecords, 1)
				if cfg.maxBuildRecords > 0 && currentCount > cfg.maxBuildRecords {
					buildErrOnce.Do(func() {
						limitErr := gxoerrors.NewTaskExecutionError("stream:join", fmt.Errorf("limit exceeded: max_build_records of %d reached", cfg.maxBuildRecords))
						log.Errorf("%v", limitErr)
						buildErr = limitErr
						cancelBuild()
					})
					continue
				}

				key, keyErr := extractKey(renderer, sCfg, record, state)
				if keyErr != nil {
					atomic.AddInt64(&stats.recordsSkipped, 1)
					select {
					case errChan <- gxoerrors.NewRecordProcessingError("stream:join", record, keyErr):
					case <-buildCtx.Done():
						return
					}
					continue
				}

				select {
				case mergedBuildChan <- buildRecord{StreamName: sCfg.StreamName, Record: record, Key: key}:
				case <-buildCtx.Done():
					return
				}
			}
		}(streamConfig)
	}

	go func() {
		buildWg.Wait()
		close(mergedBuildChan)
	}()

	for bRec := range mergedBuildChan {
		if buildErr != nil {
			continue
		}
		if buildMap[bRec.Key] == nil {
			buildMap[bRec.Key] = make(map[string][]*buildEntry)
		}
		buildMap[bRec.Key][bRec.StreamName] = append(buildMap[bRec.Key][bRec.StreamName], &buildEntry{record: bRec.Record})
	}

	return buildMap, stats, buildErr
}

func (m *JoinModule) runProbePhase(ctx context.Context, cfg *moduleConfig, renderer intTemplate.Renderer, state state.StateReader, buildMap map[interface{}]map[string][]*buildEntry, outputChans []chan<- map[string]interface{}, errChan chan<- error, log gxolog.Logger) (*probeStats, error) {
	stats := &probeStats{}
	probeStreamCfg := cfg.probeStream

	for probeRecord := range probeStreamCfg.inputChannel {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		atomic.AddInt64(&stats.probeRecordsProcessed, 1)

		probeKey, keyErr := extractKey(renderer, probeStreamCfg, probeRecord, state)
		if keyErr != nil {
			atomic.AddInt64(&stats.recordsSkipped, 1)
			select {
			case errChan <- gxoerrors.NewRecordProcessingError("stream:join", probeRecord, keyErr):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		matchedBuildStreams, found := buildMap[probeKey]
		if found {
			for _, buildEntries := range matchedBuildStreams {
				for _, entry := range buildEntries {
					entry.matched = true
				}
			}
			if err := m.emitJoinedRecords(ctx, cfg, probeRecord, matchedBuildStreams, &stats.recordsJoined, outputChans); err != nil {
				log.Errorf("Error emitting joined records: %v", err)
				return nil, err
			}
		} else if cfg.joinType == "left" || cfg.joinType == "outer" {
			if err := m.emitJoinedRecords(ctx, cfg, probeRecord, nil, &stats.recordsJoined, outputChans); err != nil {
				log.Errorf("Error emitting unmatched left/outer record: %v", err)
				return nil, err
			}
		}
	}
	return stats, nil
}

func (m *JoinModule) runPostProbePhase(ctx context.Context, cfg *moduleConfig, buildMap map[interface{}]map[string][]*buildEntry, outputChans []chan<- map[string]interface{}) (*postProbeStats, error) {
	stats := &postProbeStats{}
	if cfg.joinType != "right" && cfg.joinType != "outer" {
		return stats, nil
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var sortedKeys []interface{}
	for k := range buildMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool { return fmt.Sprintf("%v", sortedKeys[i]) < fmt.Sprintf("%v", sortedKeys[j]) })

	for _, key := range sortedKeys {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		buildStreamsForKey := buildMap[key]
		var keyWasMatched bool
	CheckMatchLoop:
		for _, entries := range buildStreamsForKey {
			for _, entry := range entries {
				if entry.matched {
					keyWasMatched = true
					break CheckMatchLoop
				}
			}
		}

		if !keyWasMatched {
			if err := m.emitJoinedRecords(ctx, cfg, nil, buildStreamsForKey, &stats.recordsJoined, outputChans); err != nil {
				return nil, fmt.Errorf("error emitting unmatched right/outer record for key %v: %w", key, err)
			}
		}
	}
	return stats, nil
}

func (m *JoinModule) emitJoinedRecords(ctx context.Context, cfg *moduleConfig, probeRecord map[string]interface{}, buildStreams map[string][]*buildEntry, recordsJoined *int64, outputChans []chan<- map[string]interface{}) error {
	combinations := getCombinations(cfg, probeRecord, buildStreams)
	if len(combinations) == 0 {
		return nil
	}
	for _, combo := range combinations {
		joinedRecord := constructJoinedRecord(cfg, combo)
		atomic.AddInt64(recordsJoined, 1)
		for _, out := range outputChans {
			select {
			case out <- joinedRecord:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func getCombinations(cfg *moduleConfig, probeRecord map[string]interface{}, buildStreams map[string][]*buildEntry) []map[string]interface{} {
	var combinations []map[string]interface{}
	if probeRecord != nil {
		combinations = []map[string]interface{}{{cfg.probeStream.StreamName: probeRecord}}
	} else {
		combinations = []map[string]interface{}{{}}
	}
	for _, buildCfg := range cfg.buildStreams {
		newCombinations := []map[string]interface{}{}
		buildEntries := buildStreams[buildCfg.StreamName]
		if len(buildEntries) == 0 {
			if cfg.joinType == "inner" {
				return nil
			}
			for _, combo := range combinations {
				newCombo := make(map[string]interface{}, len(combo)+1)
				for k, v := range combo {
					newCombo[k] = v
				}
				newCombo[buildCfg.StreamName] = nil
				newCombinations = append(newCombinations, newCombo)
			}
		} else {
			for _, combo := range combinations {
				for _, buildEntry := range buildEntries {
					newCombo := make(map[string]interface{}, len(combo)+1)
					for k, v := range combo {
						newCombo[k] = v
					}
					newCombo[buildCfg.StreamName] = buildEntry.record
					newCombinations = append(newCombinations, newCombo)
				}
			}
		}
		combinations = newCombinations
	}
	return combinations
}

func constructJoinedRecord(cfg *moduleConfig, combination map[string]interface{}) map[string]interface{} {
	joined := make(map[string]interface{})
	if cfg.mergeStrategy == "nested" {
		for streamName, record := range combination {
			nestingKey, ok := cfg.nestingKeys[streamName]
			if !ok {
				continue
			}
			joined[nestingKey] = record
		}
	} else {
		streamNames := make([]string, 0, len(combination))
		for name := range combination {
			streamNames = append(streamNames, name)
		}
		sort.Strings(streamNames)
		for _, streamName := range streamNames {
			record := combination[streamName]
			if recMap, ok := record.(map[string]interface{}); ok {
				for key, value := range recMap {
					joined[key] = value
				}
			}
		}
	}
	return joined
}

func parseAndValidateParams(params map[string]interface{}, inputs map[string]<-chan map[string]interface{}) (*moduleConfig, error) {
	cfg := moduleConfig{mergeStrategy: "flat", maxBuildRecords: 0}
	producerIDMap, _, err := paramutil.GetOptionalMap(params, module.ProducerIDMapKey)
	if err != nil {
		return nil, gxoerrors.NewValidationError("internal error: could not read _gxo_producer_id_map_", err)
	}
	cfg.joinType, err = paramutil.GetRequiredString(params, "join_type")
	if err != nil {
		return nil, err
	}
	if !(cfg.joinType == "inner" || cfg.joinType == "left" || cfg.joinType == "right" || cfg.joinType == "outer") {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("unsupported join_type: '%s'", cfg.joinType), nil)
	}
	rawOn, exists := params["on"]
	if !exists {
		return nil, gxoerrors.NewValidationError("missing required parameter 'on'", nil)
	}
	onList, ok := rawOn.([]interface{})
	if !ok || len(onList) < 2 || len(onList) != len(inputs) {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("parameter 'on' must be a list of %d objects, one for each input stream", len(inputs)), nil)
	}
	allStreamConfigs := make(map[string]joinKeyConfig)
	var probeCount int
	for i, item := range onList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("element %d in 'on' is not a map", i), nil)
		}
		streamName, err := paramutil.GetRequiredString(itemMap, "stream")
		if err != nil {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("missing 'stream' key in element %d of 'on'", i), nil)
		}
		if _, exists := allStreamConfigs[streamName]; exists {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("duplicate stream name '%s' in 'on'", streamName), nil)
		}
		role, roleExists, err := paramutil.GetOptionalString(itemMap, "role")
		if err != nil || !roleExists {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("stream '%s' must have 'role' ('build' or 'probe')", streamName), nil)
		}
		isProbe := role == "probe"
		if isProbe {
			probeCount++
		}
		producerID, idOk := producerIDMap[streamName].(string)
		if !idOk {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("internal error: producer ID for stream '%s' not found", streamName), nil)
		}
		inputChan, chanExists := inputs[producerID]
		if !chanExists {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("stream '%s' defined but no input channel provided by engine", streamName), nil)
		}
		field, fieldExists, _ := paramutil.GetOptionalString(itemMap, "field")
		template, templateExists, _ := paramutil.GetOptionalString(itemMap, "template")
		if !(fieldExists || templateExists) || (fieldExists && templateExists) {
			return nil, gxoerrors.NewValidationError(fmt.Sprintf("stream '%s' must have exactly one of 'field' or 'template'", streamName), nil)
		}
		allStreamConfigs[streamName] = joinKeyConfig{StreamName: streamName, Field: field, Template: template, IsProbe: isProbe, inputChannel: inputChan}
	}
	if probeCount != 1 {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("exactly one stream must have role 'probe', found %d", probeCount), nil)
	}
	for _, streamCfg := range allStreamConfigs {
		if streamCfg.IsProbe {
			cfg.probeStream = streamCfg
		} else {
			cfg.buildStreams = append(cfg.buildStreams, streamCfg)
		}
	}
	if outputParams, exists, _ := paramutil.GetOptionalMap(params, "output"); exists {
		if val, ok, _ := paramutil.GetOptionalString(outputParams, "merge_strategy"); ok {
			if !(val == "flat" || val == "nested") {
				return nil, gxoerrors.NewValidationError(fmt.Sprintf("unsupported merge_strategy: '%s'", val), nil)
			}
			cfg.mergeStrategy = val
		}
		if cfg.mergeStrategy == "nested" {
			nestingKeys, nkExists, _ := paramutil.GetOptionalMap(outputParams, "nesting_keys")
			if !nkExists {
				return nil, gxoerrors.NewValidationError("'nesting_keys' is required for nested merge_strategy", nil)
			}
			cfg.nestingKeys = make(map[string]string)
			for k, v := range nestingKeys {
				vs, ok := v.(string)
				if !ok {
					return nil, gxoerrors.NewValidationError("values in 'nesting_keys' must be strings", nil)
				}
				cfg.nestingKeys[k] = vs
			}
			for _, streamCfg := range allStreamConfigs {
				if _, ok := cfg.nestingKeys[streamCfg.StreamName]; !ok {
					return nil, gxoerrors.NewValidationError(fmt.Sprintf("missing nesting_key for stream '%s'", streamCfg.StreamName), nil)
				}
			}
		}
	}
	if limitsParams, exists, _ := paramutil.GetOptionalMap(params, "limits"); exists {
		max, maxExists, err := paramutil.GetOptionalInt(limitsParams, "max_build_records")
		if err != nil {
			return nil, err
		}
		if maxExists {
			cfg.maxBuildRecords = int64(max)
		}
	}
	return &cfg, nil
}

func normalizeKey(originalKey interface{}) (interface{}, error) {
	if originalKey == nil {
		return nil, nil
	}
	switch v := originalKey.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		if float32(int64(v)) == v {
			return int64(v), nil
		}
		return nil, fmt.Errorf("cannot use non-integer float32 '%f' as a join key", v)
	case float64:
		if float64(int64(v)) == v {
			return int64(v), nil
		}
		return nil, fmt.Errorf("cannot use non-integer float64 '%f' as a join key", v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, nil
		}
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func extractKey(renderer intTemplate.Renderer, cfg joinKeyConfig, record map[string]interface{}, state state.StateReader) (interface{}, error) {
	var rawKey interface{}
	var err error
	if cfg.Field != "" {
		var exists bool
		rawKey, exists = record[cfg.Field]
		if !exists {
			return nil, fmt.Errorf("join key field '%s' not found in record", cfg.Field)
		}
	} else {
		fullContext := state.GetAll()
		for k, v := range record {
			fullContext[k] = v
		}
		// The renderer passed in now correctly handles its own funcs.
		rawKey, err = renderer.Resolve(cfg.Template, fullContext)
		if err != nil {
			return nil, fmt.Errorf("failed to render join key template: %w", err)
		}
	}
	return normalizeKey(rawKey)
}