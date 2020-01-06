/* SPDX-FileCopyrightText: © 2019-2020 Nadim Kobeissi <nadim@symbolic.software>
 * SPDX-License-Identifier: GPL-3.0-only */
// 458871bd68906e9965785ac87c2708ec

package verifpal

import (
	"fmt"
	"os"
	"time"
)

// Verify runs the main verification engine for Verifpal on a model loaded from a file.
func Verify(modelFile string) []VerifyResult {
	m, valKnowledgeMap, valPrincipalStates := parserParseModel(modelFile)
	var VerifyResults []VerifyResult
	prettyMessage(fmt.Sprintf(
		"Verification initiated at %s.",
		time.Now().Format("15:04:05"),
	), 0, 0, "verifpal")
	switch m.attacker {
	case "passive":
		VerifyResults = verifyPassive(m, valKnowledgeMap, valPrincipalStates)
	case "active":
		VerifyResults = verifyActive(m, valKnowledgeMap, valPrincipalStates)
	default:
		errorCritical(fmt.Sprintf("invalid attacker (%s)", m.attacker))
	}
	fmt.Fprint(os.Stdout, "\n")
	for _, VerifyResult := range VerifyResults {
		prettyMessage(fmt.Sprintf(
			"%s: %s",
			prettyQuery(VerifyResult.query),
			VerifyResult.summary,
		), 0, 0, "result")
	}
	prettyMessage(fmt.Sprintf(
		"Verification completed at %s. Thank you for using Verifpal.",
		time.Now().Format("15:04:05"),
	), 0, 0, "verifpal")
	return VerifyResults
}

func verifyResolveQueries(
	m *Model,
	valKnowledgeMap *knowledgeMap, valPrincipalState *principalState,
	valAttackerState *attackerState, VerifyResults *[]VerifyResult, analysis int,
) {
	for q, query := range m.queries {
		if m.queries[q].resolved {
			continue
		}
		VerifyResult := queryStart(query, valAttackerState, valPrincipalState, valKnowledgeMap)
		if !VerifyResult.query.resolved {
			continue
		}
		m.queries[q].resolved = true
		*VerifyResults = append(*VerifyResults, VerifyResult)
		prettyMessage(fmt.Sprintf(
			"%s: %s",
			prettyQuery(VerifyResult.query),
			VerifyResult.summary,
		), analysis, 0, "result")
	}
}

func verifyPassive(m *Model, valKnowledgeMap *knowledgeMap, valPrincipalStates []*principalState) []VerifyResult {
	var VerifyResults []VerifyResult
	valAttackerState := constructAttackerState(false, m, valKnowledgeMap, true)
	prettyMessage("Attacker is configured as passive.", 0, 0, "info")
	valPrincipalStates[0] = sanityResolveAllPrincipalStateValues(valPrincipalStates[0], valKnowledgeMap)
	failedRewrites, _ := sanityPerformAllRewrites(valPrincipalStates[0])
	sanityFailOnFailedRewrite(failedRewrites)
	for _, a := range valPrincipalStates[0].assigned {
		sanityCheckEquationGenerators(a, valPrincipalStates[0])
	}
	verifyAnalysis(m, valPrincipalStates[0], valAttackerState, 0, 0)
	verifyResolveQueries(m, valKnowledgeMap, valPrincipalStates[0], valAttackerState, &VerifyResults, 0)
	return VerifyResults
}
