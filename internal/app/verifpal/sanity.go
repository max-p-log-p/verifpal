/*
 * SPDX-FileCopyrightText: © 2019-2020 Nadim Kobeissi <nadim@symbolic.software>
 *
 * SPDX-License-Identifier: GPL-3.0-only
 */

// 274578ab4bbd4d70871016e78cd562ad

package main

import (
	"fmt"
	"strings"
)

func sanity(model *verifpal) (*knowledgeMap, []*principalState) {
	var valKnowledgeMap *knowledgeMap
	principals := sanityDeclaredPrincipals(model)
	valKnowledgeMap = constructKnowledgeMap(model, principals)
	valPrincipalStates := constructPrincipalStates(model, valKnowledgeMap)
	sanityQueries(model, valKnowledgeMap, valPrincipalStates)
	return valKnowledgeMap, valPrincipalStates
}

func sanityAssignmentConstants(right value, constants []constant, valKnowledgeMap *knowledgeMap) []constant {
	switch right.kind {
	case "constant":
		unique := true
		for _, c := range constants {
			if right.constant.name == c.name {
				unique = false
				break
			}
		}
		if unique {
			constants = append(constants, right.constant)
		}
	case "primitive":
		p := primitiveGet(right.primitive.name)
		if len(p.name) == 0 {
			errorCritical(fmt.Sprintf(
				"invalid primitive (%s)",
				right.primitive.name,
			))
		}
		if (len(right.primitive.arguments) == 0) || ((p.arity >= 0) && (len(right.primitive.arguments) != p.arity)) {
			plural := ""
			arity := fmt.Sprintf("%d", p.arity)
			if len(right.primitive.arguments) > 1 {
				plural = "s"
			}
			if p.arity < 0 {
				arity = "at least 1"
			}
			errorCritical(fmt.Sprintf(
				"primitive %s has %d input%s, expecting %s",
				right.primitive.name, len(right.primitive.arguments), plural, arity,
			))
		}
		for _, a := range right.primitive.arguments {
			switch a.kind {
			case "constant":
				unique := true
				for _, c := range constants {
					if a.constant.name == c.name {
						unique = false
						break
					}
				}
				if unique {
					constants = append(constants, a.constant)
				}
			case "primitive":
				constants = sanityAssignmentConstants(a, constants, valKnowledgeMap)
			case "equation":
				constants = sanityAssignmentConstants(a, constants, valKnowledgeMap)
			}
		}
	case "equation":
		for _, v := range right.equation.constants {
			unique := true
			for _, c := range constants {
				if v.name == c.name {
					unique = false
					break
				}
			}
			if unique {
				constants = append(constants, v)
			}
		}
	}
	return constants
}

func sanityQueries(model *verifpal, valKnowledgeMap *knowledgeMap, valPrincipalStates []*principalState) {
	for _, query := range model.queries {
		switch query.kind {
		case "confidentiality":
			i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, query.constant)
			if i < 0 {
				errorCritical(fmt.Sprintf(
					"confidentiality query (%s) refers to unknown value (%s)",
					prettyQuery(query),
					prettyConstant(query.constant),
				))
			}
		case "authentication":
			if len(query.message.constants) != 1 {
				errorCritical(fmt.Sprintf(
					"authentication query (%s) has more than one constant",
					prettyQuery(query),
				))
			}
			c := query.message.constants[0]
			i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
			if i < 0 {
				errorCritical(fmt.Sprintf(
					"authentication query refers to unknown constant (%s)",
					prettyConstant(c),
				))
			}
			senderKnows := false
			recipientKnows := false
			if valKnowledgeMap.creator[i] == query.message.sender {
				senderKnows = true
			}
			if valKnowledgeMap.creator[i] == query.message.recipient {
				recipientKnows = true
			}
			for _, m := range valKnowledgeMap.knownBy[i] {
				if _, ok := m[query.message.sender]; ok {
					senderKnows = true
				}
				if _, ok := m[query.message.recipient]; ok {
					recipientKnows = true
				}
			}
			if !senderKnows {
				errorCritical(fmt.Sprintf(
					"authentication query (%s) depends on %s sending a constant (%s) that they do not know",
					prettyQuery(query),
					query.message.sender,
					prettyConstant(c),
				))
			}
			if !recipientKnows {
				errorCritical(fmt.Sprintf(
					"authentication query (%s) depends on %s receiving a constant (%s) that they never receive",
					prettyQuery(query),
					query.message.recipient,
					prettyConstant(c),
				))
			}
			if !sanityConstantIsUsedByPrincipal(valPrincipalStates[0], query.message.recipient, c) {
				errorCritical(fmt.Sprintf(
					"authentication query (%s) depends on %s using (%s) in a primitive, but this never happens",
					prettyQuery(query),
					query.message.recipient,
					prettyConstant(c),
				))
			}
		}
	}
}

func sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap *knowledgeMap, c constant) int {
	var index int
	found := false
	for i := range valKnowledgeMap.constants {
		if valKnowledgeMap.constants[i].name == c.name {
			found = true
			index = i
			break
		}
	}
	if !found {
		index = -1
	}
	return index
}

func sanityGetPrincipalStateIndexFromConstant(valPrincipalState *principalState, c constant) int {
	var index int
	found := false
	for i := range valPrincipalState.constants {
		if valPrincipalState.constants[i].name == c.name {
			found = true
			index = i
			break
		}
	}
	if !found {
		index = -1
	}
	return index
}

func sanityGetAttackerStateIndexFromConstant(valAttackerState *attackerState, c constant) int {
	for i, cc := range valAttackerState.known {
		switch cc.kind {
		case "constant":
			if cc.constant.name == c.name {
				return i
			}
		}
	}
	return -1
}

func sanityDeclaredPrincipals(model *verifpal) []string {
	var principals []string
	for _, block := range model.blocks {
		switch block.kind {
		case "principal":
			principals, _ = appendUnique(principals, block.principal.name)
		case "message":
			if !strInSlice(block.message.sender, principals) {
				errorCritical(fmt.Sprintf(
					"principal does not exist (%s)",
					block.message.sender,
				))
			}
			if !strInSlice(block.message.recipient, principals) {
				errorCritical(fmt.Sprintf(
					"principal does not exist (%s)",
					block.message.recipient,
				))
			}
		}
	}
	for _, query := range model.queries {
		switch query.kind {
		case "authentication":
			if !strInSlice(query.message.sender, principals) {
				errorCritical(fmt.Sprintf(
					"principal does not exist (%s)",
					query.message.sender,
				))
			}
			if !strInSlice(query.message.recipient, principals) {
				errorCritical(fmt.Sprintf(
					"principal does not exist (%s)",
					query.message.recipient,
				))
			}
		}
	}
	return principals
}

func sanityExactSameValue(a1 value, a2 value) bool {
	return strings.Compare(prettyValue(a1), prettyValue(a2)) == 0
}

func sanityEquivalentValues(a1 value, a2 value, fbm1 bool, fbm2 bool, valPrincipalState *principalState) bool {
	switch a1.kind {
	case "constant":
		a1 = sanityResolveConstant(valPrincipalState, a1.constant, fbm1)
	}
	switch a2.kind {
	case "constant":
		a2 = sanityResolveConstant(valPrincipalState, a2.constant, fbm2)
	}
	switch a1.kind {
	case "constant":
		switch a2.kind {
		case "constant":
			if a1.constant.name != a2.constant.name {
				return false
			}
			if a1.constant.output != a2.constant.output {
				return false
			}
		case "primitive":
			return false
		case "equation":
			return false
		}
	case "primitive":
		switch a2.kind {
		case "constant":
			return false
		case "primitive":
			return sanityEquivalentPrimitives(a1.primitive, a2.primitive, valPrincipalState)
		case "equation":
			return false
		}
	case "equation":
		switch a2.kind {
		case "constant":
			return false
		case "primitive":
			return false
		case "equation":
			return sanityEquivalentEquations(a1.equation, a2.equation, valPrincipalState)
		}
	}
	return true
}

func sanityEquivalentPrimitives(p1 primitive, p2 primitive, valPrincipalState *principalState) bool {
	if p1.name != p2.name {
		return false
	}
	if len(p1.arguments) != len(p2.arguments) {
		return false
	}
	for i := range p1.arguments {
		equiv := sanityEquivalentValues(p1.arguments[i], p2.arguments[i], false, false, valPrincipalState)
		if !equiv {
			return false
		}
	}
	return true
}

func sanityDeconstructEquationValues(e equation, valPrincipalState *principalState) []value {
	var values []value
	for _, c := range e.constants {
		values = append(values, sanityResolveConstant(valPrincipalState, c, false))
	}
	return values
}

func sanityFindConstantInPrimitive(c constant, p primitive, valPrincipalState *principalState) bool {
	for _, a := range p.arguments {
		switch a.kind {
		case "constant":
			if sanityEquivalentValues(a, value{kind: "constant", constant: c}, false, false, valPrincipalState) {
				return true
			}
			a = sanityResolveConstant(valPrincipalState, a.constant, false)
		}
		switch a.kind {
		case "constant":
			if sanityEquivalentValues(a, value{kind: "constant", constant: c}, false, false, valPrincipalState) {
				return true
			}
		case "primitive":
			if sanityFindConstantInPrimitive(c, a.primitive, valPrincipalState) {
				return true
			}
		case "equation":
			v := sanityDeconstructEquationValues(a.equation, valPrincipalState)
			for _, vv := range v {
				if sanityEquivalentValues(vv, value{kind: "constant", constant: c}, false, false, valPrincipalState) {
					return true
				}
			}
		}
	}
	return false
}

func sanityEquivalentEquations(e1 equation, e2 equation, valPrincipalState *principalState) bool {
	e1Values := sanityDeconstructEquationValues(e1, valPrincipalState)
	e2Values := sanityDeconstructEquationValues(e2, valPrincipalState)
	if (len(e1Values) == 0) || (len(e2Values) == 0) {
		return false
	}
	if len(e1Values) != len(e2Values) {
		return false
	}
	if e1Values[0].kind == "equation" && e2Values[0].kind == "equation" {
		e1Base := sanityDeconstructEquationValues(e1Values[0].equation, valPrincipalState)
		e2Base := sanityDeconstructEquationValues(e2Values[0].equation, valPrincipalState)
		if sanityEquivalentValues(e1Base[1], e2Values[1], false, false, valPrincipalState) && sanityEquivalentValues(e1Values[1], e2Base[1], false, false, valPrincipalState) {
			return true
		}
		if sanityEquivalentValues(e1Base[1], e2Base[1], false, false, valPrincipalState) && sanityEquivalentValues(e1Values[1], e2Values[1], false, false, valPrincipalState) {
			return true
		}
		return false
	}
	if len(e1Values) > 2 {
		if sanityEquivalentValues(e1Values[1], e2Values[2], false, false, valPrincipalState) {
			if sanityEquivalentValues(e1Values[2], e2Values[1], false, false, valPrincipalState) {
				return true
			}
		}
		if sanityEquivalentValues(e1Values[1], e2Values[1], false, false, valPrincipalState) {
			if sanityEquivalentValues(e1Values[2], e2Values[2], false, false, valPrincipalState) {
				return true
			}
		}
	}
	if sanityEquivalentValues(e1Values[0], e2Values[0], false, false, valPrincipalState) {
		if sanityEquivalentValues(e1Values[1], e2Values[1], false, false, valPrincipalState) {
			return true
		}
	}
	return false
}

func sanityExactSameValueInValues(v value, assigneds *[]value) int {
	index := -1
	for i, a := range *assigneds {
		if sanityExactSameValue(v, a) {
			index = i
			break
		}
	}
	return index
}

func sanityValueInValues(v value, assigneds *[]value, valPrincipalState *principalState) int {
	index := -1
	for i, a := range *assigneds {
		if sanityEquivalentValues(v, a, false, false, valPrincipalState) {
			index = i
			break
		}
	}
	return index
}

func sanityPerformRewrites(valPrincipalState *principalState) ([]primitive, []int) {
	var failedRewrites []primitive
	var failedRewritesIndices []int
	for i, c := range valPrincipalState.constants {
		a := sanityResolveConstant(valPrincipalState, c, false)
		switch a.kind {
		case "constant":
			continue
		case "primitive":
			prim := primitiveGet(a.primitive.name)
			if prim.rewrite.hasRule {
				wasRewritten, rewrite := possibleToPrimitivePassRewrite(a.primitive, valPrincipalState)
				if wasRewritten {
					valPrincipalState.wasRewritten[i] = true
					valPrincipalState.assigned[i] = rewrite
					valPrincipalState.beforeMutate[i] = rewrite
				} else {
					failedRewrites = append(failedRewrites, a.primitive)
					failedRewritesIndices = append(failedRewritesIndices, i)
				}
			}
		case "equation":
			continue
		}
	}
	return failedRewrites, failedRewritesIndices
}

func sanityCheckEquationRootGenerator(e equation, ee equation, valPrincipalState *principalState) {
	a, _ := sanityResolveInternalValues(value{kind: "equation", equation: e}, valPrincipalState, false)
	if len(a.equation.constants) > 3 {
		errorCritical(fmt.Sprintf(
			"too many layers in equation (%s), maximum is 2",
			prettyEquation(e),
		))
	}
	for i, c := range a.equation.constants {
		if i == 0 {
			if c.name != "g" {
				errorCritical(fmt.Sprintf(
					"equation (%s) does not use 'g' as generator",
					prettyEquation(e),
				))
			}
		}
		if i > 0 {
			if c.name == "g" {
				errorCritical(fmt.Sprintf(
					"equation (%s) uses 'g' not as a generator",
					prettyEquation(e),
				))
			}
		}
	}
}

func sanityCheckEquationGenerators(valPrincipalState *principalState) {
	for _, a := range valPrincipalState.assigned {
		if a.kind == "equation" {
			sanityCheckEquationRootGenerator(a.equation, a.equation, valPrincipalState)
		}
	}
}

func sanityResolveInternalValues(a value, valPrincipalState *principalState, forceBeforeMutate bool) (value, []value) {
	var v []value
	switch a.kind {
	case "constant":
		v = append(v, a)
		a = sanityResolveConstant(valPrincipalState, a.constant, forceBeforeMutate)
	}
	switch a.kind {
	case "constant":
		return a, v
	case "primitive":
		r := value{
			kind: "primitive",
			primitive: primitive{
				name:      a.primitive.name,
				arguments: []value{},
				check:     a.primitive.check,
			},
		}
		for _, aa := range a.primitive.arguments {
			s, vv := sanityResolveInternalValues(aa, valPrincipalState, forceBeforeMutate)
			v = append(v, vv...)
			r.primitive.arguments = append(r.primitive.arguments, s)
		}
		return r, v
	case "equation":
		r := value{
			kind: "equation",
			equation: equation{
				constants: []constant{},
			},
		}
		if len(a.equation.constants) > 2 {
			for _, vv := range a.equation.constants {
				v = append(v, value{kind: "constant", constant: vv})
			}
			return a, v
		}
		aa := sanityDeconstructEquationValues(a.equation, valPrincipalState)
		switch aa[0].kind {
		case "constant":
			aa[0] = sanityResolveConstant(valPrincipalState, aa[0].constant, forceBeforeMutate)
		}
		switch aa[0].kind {
		case "constant":
			r.equation.constants = append(r.equation.constants, aa[0].constant)
		case "primitive":
			r.equation.constants = append(r.equation.constants, aa[0].constant)
		case "equation":
			aaa, _ := sanityResolveInternalValues(aa[0], valPrincipalState, forceBeforeMutate)
			r.equation.constants = aaa.equation.constants
		}
		switch aa[1].kind {
		case "constant":
			r.equation.constants = append(r.equation.constants, aa[1].constant)
		case "primitive":
			r.equation.constants = append(r.equation.constants, aa[1].constant)
		case "equation":
			aaa, _ := sanityResolveInternalValues(aa[1], valPrincipalState, forceBeforeMutate)
			r.equation.constants = append(r.equation.constants, aaa.equation.constants[1:]...)
		}
		for _, vv := range r.equation.constants {
			v = append(v, value{kind: "constant", constant: vv})
		}
		return r, v
	}
	return a, v
}

func sanityResolveConstant(valPrincipalState *principalState, c constant, forceBeforeMutate bool) value {
	i := sanityGetPrincipalStateIndexFromConstant(valPrincipalState, c)
	if i < 0 {
		return value{kind: "constant", constant: c}
	}
	if forceBeforeMutate {
		return valPrincipalState.beforeMutate[i]
	}
	if valPrincipalState.wasMutated[i] {
		if valPrincipalState.creator[i] == valPrincipalState.name {
			return valPrincipalState.beforeMutate[i]
		}
		if !valPrincipalState.known[i] {
			return valPrincipalState.beforeMutate[i]
		}
	}
	return valPrincipalState.assigned[i]
}

func sanityConstantIsUsedByPrincipal(valPrincipalState *principalState, name string, c constant) bool {
	for i, a := range valPrincipalState.beforeRewrite {
		if valPrincipalState.creator[i] != name {
			continue
		}
		switch a.kind {
		case "primitive":
			_, v := sanityResolveInternalValues(a, valPrincipalState, false)
			for _, vv := range v {
				switch vv.kind {
				case "constant":
					if vv.constant.name == c.name {
						return true
					}
				}
			}
		}
	}
	return false
}
