/* SPDX-FileCopyrightText: © 2019-2020 Nadim Kobeissi <nadim@symbolic.software>
 * SPDX-License-Identifier: GPL-3.0-only */
// 806d8db3ce9f3ded40fd35fdba02fb84
package verifpal

import (
	"fmt"
	"os"
	"strings"
)

// Coq translates a Verifpal model into a representation that fits
// into the Coq model of the Verifpal verification methodology.
func Coq(modelFile string) {
	m := parserParseModel(modelFile, false)
	sanity(m)
	fmt.Fprint(os.Stdout, coqModel(m))
}
func coqModel(m Model) string {
	output := []string{}
	names := make(map[string]int)
	messageLog := make(map[string]string)
	names["kmap"] = 0
	names["unnamed"] = 0
	output = append(output, coqHeader())
	// attacker := m.attacker
	output = append(output, fmt.Sprintf("\n\n\n\t(* Protocol: %s *)\n", m.fileName))
	for _, block := range m.blocks {
		switch block.kind {
		case "principal":
			{
				output, names = coqPrincipalBlock(block, output, names)
			}
		case "message":
			{
				output, names, messageLog = coqMessageBlock(block.message, output, names, messageLog)
			}
		}
	}
	return strings.Join(output, "\n")
}

func coqPrincipalBlock(block block, output []string, names map[string]int) ([]string, map[string]int) {
	if names["kmap"] == 0 {
		output = append(output, fmt.Sprintf(
			"Definition kmap_%d := knowledgemap_constructor \"%s\".",
			names["kmap"], block.principal.name,
		))
		names["kmap"]++
	} else if _, isFound := names[(strings.ToLower(block.principal.name))]; !isFound {
		names[(strings.ToLower(block.principal.name))] = 0
		output = append(output, fmt.Sprintf(
			"Definition kmap_%d := add_principal_knowledgemap kmap_%d \"%s\".",
			names["kmap"], names["kmap"]-1, block.principal.name,
		))
		names["kmap"]++
	}
	for i, expression := range block.principal.expressions {
		if i == 0 {
			output = append(output, fmt.Sprintf(
				"Definition %s_%d := get_principal_knowledgemap kmap_%d \"%s\".",
				strings.ToLower(block.principal.name), names[(strings.ToLower(block.principal.name))],
				names["kmap"]-1, block.principal.name,
			))
			names[(strings.ToLower(block.principal.name))]++
		}
		output, names = coqExpressionBlock(expression, strings.ToLower(block.principal.name), output, names)
		if i == len(block.principal.expressions)-1 {
			output = append(output, fmt.Sprintf(
				"Definition kmap_%d := update_principal_knowledgemap kmap_%d %s_%d.",
				names["kmap"], (names["kmap"]-1),
				strings.ToLower(block.principal.name), (names[(strings.ToLower(block.principal.name))]-1),
			))
			names["kmap"]++
		}
	}
	return output, names
}

func coqMessageBlock(message message, output []string, names map[string]int, messageLog map[string]string) ([]string, map[string]int, map[string]string) {
	for _, constant := range message.constants {
		output = append(output, fmt.Sprintf(
			"Definition kmap_%d := add_message_knowledgemap kmap_%d (message_constructor \"%s\" \"%s\" \"%s\" %s).",
			names["kmap"], (names["kmap"]-1),
			message.sender, message.recipient, constant.name, coqGuard(constant.guard),
		))
		names["kmap"]++
		output = append(output, fmt.Sprintf(
			"Definition kmap_%d := send_message kmap_%d.",
			names["kmap"], (names["kmap"]-1),
		))
		messageLog[constant.name] = fmt.Sprintf("kmap_%d", names["kmap"])
		names["kmap"]++
	}
	return output, names, messageLog
}

func coqExpressionBlock(expression expression, principalName string, output []string, names map[string]int) ([]string, map[string]int) {
	switch expression.kind {
	case "knows":
		{
			for _, constant := range expression.constants {
				output = append(output, fmt.Sprintf(
					"Definition %s_%d := know_value %s_%d \"%s\" %s.",
					principalName, names["kmap"],
					principalName, names["kmap"]-1,
					constant.name, expression.qualifier,
				))
				names["kmap"]++
			}
		}
	case "generates":
		{
			for _, constant := range expression.constants {
				output = append(output, fmt.Sprintf(
					"Definition %s_%d := generate_value %s_%d \"%s\".",
					principalName, names["kmap"],
					principalName, names["kmap"]-1,
					constant.name,
				))
				names["kmap"]++
			}
		}
	case "leaks":
		{
			for _, constant := range expression.constants {
				output = append(output, fmt.Sprintf(
					"Definition %s_%d := leak_value %s_%d \"%s\".",
					principalName, names["kmap"],
					principalName, names["kmap"]-1,
					constant.name,
				))
				names["kmap"]++
			}
		}
	case "assignment":
		{
			update := ""
			for n, e := range expression.left {
				update, output, names = coqValue(expression.right, principalName, n+1, output, names)
				output = append(output, fmt.Sprintf(
					"Definition %s_%d := assign_value %s_%d %s \"%s\".",
					principalName, names["kmap"],
					principalName, names["kmap"]-1,
					update, e.name,
				))
				names["kmap"]++
			}
		}
	}
	return output, names
}

func coqValue(v value, principalName string, n int, output []string, names map[string]int) (string, []string, map[string]int) {
	update := ""
	switch v.kind {
	case "constant":
		{
			return coqConstant(v.constant.name, principalName, names), output, names
		}
		// TODO: Checked primitives
		// TODO: HASH, concat problem
	case "primitive":
		{
			update = "(" + v.primitive.name
			if v.primitive.name == "HKDF" || v.primitive.name == "SHAMIR_SPLIT" {
				if n > 3 {
					errorCritical("Only 3 outputs are allowed for " + v.primitive.name)
				} else {
					update += fmt.Sprintf("_%d", n)
				}
			}
			update += " "
			for i, argument := range v.primitive.arguments {
				if argument.kind != "constant" {
					newConstName := fmt.Sprintf("unnamed_%d", names["unnamed"])
					exp := expression{
						argument.kind,
						"private",
						[]constant{},
						[]constant{
							{
								false, false, false,
								newConstName,
								"assignment", "private",
							},
						},
						argument,
					}
					output, names = coqExpressionBlock(exp, principalName, output, names)
					update += coqConstant(newConstName, principalName, names)
					names["unnamed"]++
				} else {
					update += coqConstant(argument.constant.name, principalName, names)
				}
				if i == len(v.primitive.arguments)-1 {
					update += ")"
				} else {
					update += " "
				}
			}
		}
	case "equation":
		{
			if v.equation.values[0].constant.name == "g" {
				update = "(public_key "
			} else {
				update = "(DH " + coqConstant(v.equation.values[0].constant.name, principalName, names)
			}
			update += coqConstant(v.equation.values[1].constant.name, principalName, names) + ")"
		}
	}
	return update, output, names
}

func coqConstant(constantName string, principalName string, names map[string]int) string {
	return fmt.Sprintf(
		"(get %s_%d \"%s\")",
		principalName, names[principalName],
		constantName,
	)
}

func coqGuard(guard bool) string {
	if guard {
		return "guarded"
	}
	return "unguarded"
}

func coqHeader() string {
	return strings.Join([]string{
		"Require Import Notations Logic Datatypes PeanoNat String.",
		"Local Open Scope nat_scope.",
		"Inductive constant : Type :=",
		"\t| nil",
		"\t| value (s: string)",
		"\t(* primitive output *)",
		"\t| equation_c (base: constant) (exp: constant)",
		"\t| mult_c (c1: constant) (c2: constant)",
		"\t| ENC_c (key: constant) (message: constant)",
		"\t| AEAD_ENC_c (key: constant) (message: constant) (ad: constant)",
		"\t| PKE_ENC_c (G_key: constant) (message: constant)",
		"\t| CONCAT_c (a: constant) (b: constant)",
		"\t| HASH_c (x: constant)",
		"\t| MAC_c (key: constant) (message: constant)",
		"\t| HKDF1_c (salt: constant) (ikm: constant) (info: constant)",
		"\t| HKDF2_c (salt: constant) (ikm: constant) (info: constant)",
		"\t| HKDF3_c (salt: constant) (ikm: constant) (info: constant)",
		"\t| PW_HASH_c (x: constant)",
		"\t| SIGN_c (k: constant) (m: constant)",
		"\t| RINGSIGN_c (key_a: constant) (G_key_b: constant) (G_key_c: constant) (message: constant)",
		"\t| SHAMIR_SPLIT1_c (k: constant)",
		"\t| SHAMIR_SPLIT2_c (k: constant)",
		"\t| SHAMIR_SPLIT3_c (k: constant)",
		"\t| SHAMIR_JOIN_c (sa: constant) (sb: constant)",
		"\t(* for checked primitives *)",
		"\t| INVALID (s: string)",
		"\t| VALID.",
		"Definition G := value \"G\".",
		"Notation \"x && y\" := (andb x y).",
		"(* Notation \"s :: c\" := (value s c)",
		"\t(at level 60, right associativity). *)",
		"(* Notation \"[ s ; .. ; c ]\" := (constant_meta_c s .. (constant_meta_c c nil) ..). *)",
		"Fixpoint equal_consts(c1 c2: constant) : bool :=",
		"\tmatch c1, c2 with",
		"\t| nil, nil => true",
		"\t| value s1, value s2 => eqb s1 s2",
		"\t| equation_c base1 exp1, equation_c base2 exp2 => andb (equal_consts base1 base2) (equal_consts exp1 exp2)",
		"\t| mult_c a b, mult_c c d => orb (andb (equal_consts a c) (equal_consts b d)) (andb (equal_consts a d) (equal_consts b c))",
		"\t| ENC_c k1 m1, ENC_c k2 m2 => andb (equal_consts k1 k2) (equal_consts m1 m2)",
		"\t| AEAD_ENC_c k1 m1 ad1, AEAD_ENC_c k2 m2 ad2 => andb (andb (equal_consts k1 k2) (equal_consts m1 m2)) (equal_consts ad1 ad2)",
		"\t| PKE_ENC_c k1 m1, PKE_ENC_c k2 m2 => andb (equal_consts k1 k2) (equal_consts m1 m2)",
		"\t| CONCAT_c a b, CONCAT_c c d => andb (equal_consts a c) (equal_consts b d)",
		"\t| HASH_c a, HASH_c b => equal_consts a b",
		"\t| MAC_c k1 m1, MAC_c k2 m2 => andb (equal_consts k1 k2) (equal_consts m1 m2)",
		"\t| PW_HASH_c a, PW_HASH_c b => equal_consts a b",
		"\t| SIGN_c k1 m1, SIGN_c k2 m2 => andb (equal_consts k1 k2) (equal_consts m1 m2)",
		"\t| RINGSIGN_c ka1 gkb1 gkc1 m1, RINGSIGN_c ka2 gkb2 gkc2 m2 =>",
		"\t\tandb (andb (equal_consts ka1 ka2) (equal_consts m1 m2)) ",
		"\t\t\t\t(orb (andb (equal_consts gkb1 gkb2) (equal_consts gkc1 gkc2))",
		"\t\t\t\t\t(andb (equal_consts gkb1 gkc2) (equal_consts gkc1 gkb2)))",
		"\t| SHAMIR_SPLIT1_c ka, SHAMIR_SPLIT1_c kb => equal_consts ka kb",
		"\t| SHAMIR_SPLIT2_c ka, SHAMIR_SPLIT2_c kb => equal_consts ka kb",
		"\t| SHAMIR_SPLIT3_c ka, SHAMIR_SPLIT3_c kb => equal_consts ka kb",
		"\t| SHAMIR_JOIN_c sa1 sb1, SHAMIR_JOIN_c sa2 sb2 => orb (andb (equal_consts sa1 sa2)(equal_consts sb1 sb2)) (andb (equal_consts sa1 sb2) (equal_consts sb1 sa2))",
		"\t| HKDF1_c salt1 ikm1 info1, HKDF1_c salt2 ikm2 info2 => andb (equal_consts salt1 salt2) (andb (equal_consts ikm1 ikm2) (equal_consts info1 info2))",
		"\t| HKDF2_c salt1 ikm1 info1, HKDF2_c salt2 ikm2 info2 => andb (equal_consts salt1 salt2) (andb (equal_consts ikm1 ikm2) (equal_consts info1 info2))",
		"\t| HKDF3_c salt1 ikm1 info1, HKDF3_c salt2 ikm2 info2 => andb (equal_consts salt1 salt2) (andb (equal_consts ikm1 ikm2) (equal_consts info1 info2))",
		"\t| _, _ => false",
		"\tend.",
		"",
		"Fixpoint multiply(c1 c2: constant) : constant :=",
		"\tmatch c1, c2 with",
		"\t| _, nil => c1",
		"\t| nil, _ => c2",
		"\t| _, _ => mult_c c1 c2",
		"\tend.",
		"",
		"Notation \"c1 * c2\" := (multiply c1 c2) (at level 40, left associativity) : nat_scope.",
		"Notation \"x =? y\" := (equal_consts x y) (at level 70) : nat_scope.",
		"(* ASSUMPTION: COMMUTATIVTY IN DH MULT*)",
		"(*need to define data type similar to set (array with no order) *)",
		"Theorem mult_associativity: forall b c: constant, b*c = c*b.",
		"\tAdmitted.",
		"Fixpoint public_key(secret: constant) : constant := equation_c G secret.",
		"Notation \" G^( c )\" := (public_key c)",
		"\t(at level 30, right associativity).",
		"Theorem pub_key: forall x: constant, public_key x = equation_c G x.",
		"Proof.",
		"\tintros x. destruct x eqn:E.",
		"\treflexivity. reflexivity. reflexivity. reflexivity. reflexivity.",
		"\treflexivity. reflexivity. reflexivity. reflexivity. reflexivity.",
		"\treflexivity. reflexivity. reflexivity. reflexivity. reflexivity.",
		"\treflexivity. reflexivity. reflexivity. reflexivity. reflexivity.",
		"\treflexivity. reflexivity.",
		"Qed.",
		"(* a private key always has the same public key *)",
		"Theorem pub_key_eq: forall x y: constant, x = y -> public_key x = public_key y.",
		"Proof.",
		"\tintros x y H.",
		"\trewrite <- H.",
		"\treflexivity.",
		"Qed.",
		"Fixpoint DH(c1 c2: constant): constant :=",
		"\tmatch c1, c2 with",
		"\t| equation_c base1 exp1, equation_c base2 exp2 => equation_c G (exp1 * exp2)",
		"\t| equation_c base exp, _ => equation_c G (exp * c2)",
		"\t| _, equation_c base exp => equation_c G (exp * c1)",
		"\t| _, _ => equation_c G (c1 * c2)",
		"\tend.",
		"Definition x := value \"x\".",
		"Definition y := value \"y\".",
		"Definition gx := public_key x.",
		"Definition gy := public_key y.",
		"Compute (DH gx y).",
		"Compute (DH gy x).",
		"Compute (equal_consts (DH gx y) (DH gy x)).",
		"(* WIP Equations *)",
		"(* Theorem dh_eq: forall x y, (DH (public_key x) y) = equation_c G (x*y).",
		"Proof.",
		"intros x y.",
		"destruct x eqn:Ex.",
		"destruct y eqn:Ey.",
		"reflexivity. reflexivity. simpl. reflexivity. *)",
		"(* Theorem dh_eq: forall x y, (DH (public_key x) y) = (DH (public_key y) x). *)",
		"(*theorem gxy = gyx*)",
		"(* Encryption Primitives *)",
		"Fixpoint ENC(key plaintext: constant): constant := ENC_c key plaintext.",
		"Fixpoint DEC(key ciphertext: constant): constant :=",
		"\tmatch ciphertext with",
		"\t| ENC_c k m => match equal_consts k key with",
		"\t\t\t\t\t| true => m",
		"\t\t\t\t\t| false => ENC_c k m",
		"\t\t\t\t\tend",
		"\t| _ => ciphertext",
		"\tend.",
		"",
		"(*ASSUMPTION*)",
		"Theorem enc_dec: forall k m: constant, DEC k (ENC k m) = m.",
		"Admitted.",
		"Theorem enc_dec_2: forall k m c: constant, c = ENC k m -> m = DEC k c.",
		"\tProof.",
		"\t\tintros k m c H.",
		"\t\trewrite -> H.",
		"\t\trewrite -> enc_dec.",
		"\t\treflexivity.",
		"\tQed.",
		"Fixpoint AEAD_ENC(key plaintext ad: constant): constant := AEAD_ENC_c key plaintext ad.",
		"Fixpoint AEAD_DEC(key ciphertext ad: constant) : constant :=",
		"\tmatch ciphertext with ",
		"\t| AEAD_ENC_c k m ad' => match equal_consts ad ad' with",
		"\t\t\t\t\t\t\t| true => match equal_consts key k with",
		"\t\t\t\t\t\t\t\t\t\t| true => m",
		"\t\t\t\t\t\t\t\t\t\t| false => ciphertext",
		"\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t| false => INVALID \"AEAD_DEC_fail_ad_mismatch\"",
		"\t\t\t\t\t\t\tend",
		"\t| _ => ciphertext",
		"\tend.",
		"",
		"Fixpoint PKE_ENC(gkey plaintext: constant) : constant := PKE_ENC_c gkey plaintext.",
		"Fixpoint PKE_DEC(key ciphertext: constant) : constant :=",
		"\tmatch ciphertext with",
		"\t| PKE_ENC_c gkey plaintext => match equal_consts (public_key key) gkey with",
		"\t\t\t\t\t\t\t\t\t| true => plaintext",
		"\t\t\t\t\t\t\t\t\t| false => ciphertext",
		"\t\t\t\t\t\t\t\t\tend",
		"\t| _ => ciphertext",
		"\tend.",
		"",
		"\t(* Hashing Primitives *)",
		"Fixpoint HASH(a: constant) : constant := HASH_c a.",
		"Fixpoint MAC(key message: constant) : constant := MAC_c key message.",
		"Fixpoint PW_HASH(a: constant) : constant := PW_HASH_c a.",
		"Fixpoint HKDF_1 (salt ikm info: constant) := HKDF1_c salt ikm info.",
		"Fixpoint HKDF_2 (salt ikm info: constant) := HKDF2_c salt ikm info.",
		"Fixpoint HKDF_3 (salt ikm info: constant) := HKDF3_c salt ikm info.",
		"\t(* Signature Primitives *)",
		"Fixpoint SIGN(key message: constant) : constant := SIGN_c key message.",
		"Fixpoint SIGNVERIF(gkey message signature: constant) : constant :=",
		"match gkey\t, signature with",
		"\t| equation_c base exp, SIGN_c key m => match andb (equal_consts exp key) (equal_consts message m) with",
		"\t\t\t\t\t\t\t\t\t\t\t| true => message",
		"\t\t\t\t\t\t\t\t\t\t\t| false => INVALID \"SIGNVERIF_fail\"",
		"\t\t\t\t\t\t\t\t\t\t\tend",
		"\t| _, _ => signature",
		"\tend.",
		"",
		"Fixpoint RINGSIGN(key_a gkey_b gkey_c message: constant) : constant := RINGSIGN_c key_a gkey_b gkey_c message.",
		"Fixpoint RINGSIGNVERIF(ga gb gc m signature: constant): constant :=",
		"\tmatch signature with",
		"\t| RINGSIGN_c key_a b c message => match ga, gb, gc with ",
		"\t\t\t\t\t\t\t\t\t| equation_c base_a exp_a, equation_c base_b exp_b, equation_c base_c exp_c",
		"\t\t\t\t\t\t\t\t\t\t=> match orb (orb (equal_consts exp_a key_a) (equal_consts exp_b key_a)) (equal_consts exp_b key_a) with",
		"\t\t\t\t\t\t\t\t\t\t\t| true => m",
		"\t\t\t\t\t\t\t\t\t\t\t| false => INVALID \"RINGSIGNVERIF_fail_unable_to_auth\"",
		"\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\t| _, _, _ =>  INVALID \"RINGSIGNVERIF_fail_key_type_mismatch\"",
		"\t\t\t\t\t\t\t\t\tend",
		"\t| _ => signature",
		"\tend.",
		"",
		"\t(* Secret Sharing Primitives *)",
		"Fixpoint SHAMIR_SPLIT_1 (k: constant) : constant := SHAMIR_SPLIT1_c k.",
		"Fixpoint SHAMIR_SPLIT_2 (k: constant) : constant := SHAMIR_SPLIT2_c k.",
		"Fixpoint SHAMIR_SPLIT_3 (k: constant) : constant := SHAMIR_SPLIT3_c k.",
		"Fixpoint SHAMIR_JOIN (sa sb: constant) : constant :=",
		"\tmatch sa,sb with",
		"\t| SHAMIR_SPLIT1_c ka, SHAMIR_SPLIT2_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| SHAMIR_SPLIT1_c ka, SHAMIR_SPLIT3_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| SHAMIR_SPLIT2_c ka, SHAMIR_SPLIT1_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| SHAMIR_SPLIT2_c ka, SHAMIR_SPLIT3_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| SHAMIR_SPLIT3_c ka, SHAMIR_SPLIT1_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| SHAMIR_SPLIT3_c ka, SHAMIR_SPLIT2_c kb => match equal_consts ka kb with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => ka",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => SHAMIR_JOIN_c sa sb",
		"\t\t\t\t\t\t\t\t\t\t\t\tend ",
		"\t| _, _ => SHAMIR_JOIN_c sa sb",
		"\tend.",
		"",
		"\t(* Core Primitives *)",
		"Fixpoint ASSERT (c1 c2: constant) : constant  :=",
		"match equal_consts \tc1 c2 with",
		"\t| true => VALID",
		"\t| false => INVALID \"ASSERT_fail\"",
		"\tend.",
		"",
		"Fixpoint length (c: constant) : nat :=",
		"\tmatch c with",
		"\t| CONCAT_c a b => length a + length b",
		"\t| _ => 1",
		"\tend.",
		"",
		"Fixpoint CONCAT (c1 c2: constant) : constant :=",
		"\tmatch ((length c1) + (length c2)) <? 5 with",
		"\t| false => INVALID \"Cannot concatenate more than 5 constants.\"",
		"\t| true => match c1 with",
		"\t\t\t\t| nil => c2",
		"\t\t\t\t| _ => CONCAT_c c1 c2",
		"\t\t\t\tend",
		"\tend.",
		"",
		"(* boolean here for rhs or lhs of concat*)",
		"Fixpoint SPLIT (c: constant) (lhs: bool): constant :=",
		"\tmatch c with",
		"\t| CONCAT_c c1 c2 => match lhs with ",
		"\t\t\t\t\t\t| true => c1",
		"\t\t\t\t\t\t| false => c2",
		"\t\t\t\t\t\tend",
		"\t| _ => c",
		"\tend.",
		"",
		"(*end of primitives*)",
		"Inductive qualifier : Type :=",
		"\t| public",
		"\t| private",
		"\t| password.",
		"Inductive declaration : Type :=",
		"\t| assignment",
		"\t| knows",
		"\t| generates.",
		"Inductive guard_state : Type :=",
		"\t| guarded",
		"\t| unguarded.",
		"Inductive leak_state : Type :=",
		"\t| leaked",
		"\t| not_leaked.",
		"Inductive constant_meta: Type :=",
		"\t| constant_meta_c (c: constant) (d: declaration) (q: qualifier) (created_by name: string) (l: leak_state)",
		"\t| constant_meta_invalid (code: string).",
		"Fixpoint constant_meta_constructor (c: constant) (d: declaration) (q: qualifier) (created_by name: string) :=",
		"\tmatch eqb created_by \"\", eqb name \"\" with",
		"\t| true, true => constant_meta_invalid \"constant_meta must have an non empty value for created_by and name.\"",
		"\t| true, false => constant_meta_invalid \"constant_meta must have an non empty value for created_by.\"",
		"\t| false, true => constant_meta_invalid \"constant_meta must have an non empty value for name.\"",
		"\t| false, false => constant_meta_c c d q created_by name not_leaked",
		"\tend.",
		"",
		"Fixpoint get_name_constant_meta (c: constant_meta) : string :=",
		"\tmatch c with",
		"\t| constant_meta_invalid code => code",
		"\t| constant_meta_c _ _ _ _ name _ => name",
		"\tend.",
		"",
		"Fixpoint equal_constant_meta (a b: constant_meta) : bool :=",
		"\tmatch a,b with",
		"\t| constant_meta_c c1 _ _ _ _ _, constant_meta_c c2 _ _ _ _ _ => equal_consts c1 c2",
		"\t| _, _ => false",
		"\tend.",
		"",
		"Fixpoint leak_constant_meta (cm: constant_meta) : constant_meta :=",
		"\tmatch cm with",
		"| constant_meta_invalid code => constant_meta_invalid (\"Attempting to leak invalid constant_meta; \" ++ code)",
		"| constant_meta_c c d q created_by name _ => constant_meta_c c d q created_by name leaked",
		"\tend.",
		"",
		"Inductive principal_knowledge: Type :=",
		"\t| principal_knowledge_empty",
		"\t| principal_knowledge_invalid (code: string)",
		"\t| principal_knowledge_c (c: constant_meta) (next: principal_knowledge).",
		"",
		"Fixpoint principal_knowledge_constructor (cm: constant_meta) (next: principal_knowledge) : principal_knowledge :=",
		"\tmatch cm with",
		"\t| constant_meta_invalid code => principal_knowledge_invalid \"Attempting to construct principal_knowledge using invalid constant_meta\"",
		"\t| constant_meta_c _ _ _ _ _ _ => match next with",
		"\t\t\t\t\t\t\t\t\t| principal_knowledge_invalid code => principal_knowledge_invalid \"Attempting to construct principal_knowledge using invalid provided next principal_knowledge\"",
		"\t\t\t\t\t\t\t\t\t| _ => principal_knowledge_c cm next",
		"\t\t\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint push_pk (pk: principal_knowledge) (cm: constant_meta) : principal_knowledge :=",
		"\tmatch pk with",
		"\t| principal_knowledge_invalid code => principal_knowledge_invalid (\"Attempting to push constant_meta to invalid principal_knowledge; \" ++ code)",
		"\t| _ => principal_knowledge_constructor cm pk",
		"\tend.",
		"",
		"Fixpoint get_constant_meta_by_name_pk (pk: principal_knowledge) (name: string) : constant_meta :=",
		"\tmatch pk with",
		"\t| principal_knowledge_invalid code => constant_meta_invalid (\"Attempting to get constant_meta from invalid principal_knowledge; \" ++ code)",
		"\t| principal_knowledge_empty => constant_meta_invalid \"Value not found\"",
		"\t| principal_knowledge_c c next => match eqb name \"\" with",
		"\t\t\t\t\t\t\t\t\t| true => constant_meta_invalid \"Attempting to get a constant_meta with an empty string as its name\"",
		"\t\t\t\t\t\t\t\t\t| false => match eqb (get_name_constant_meta c) name with",
		"\t\t\t\t\t\t\t\t\t\t\t\t| true => c",
		"\t\t\t\t\t\t\t\t\t\t\t\t| false => get_constant_meta_by_name_pk next name",
		"\t\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint remove_constant_meta_pk (pk: principal_knowledge) (name: string) : principal_knowledge :=",
		"\tmatch pk with",
		"\t| principal_knowledge_empty => pk",
		"\t| principal_knowledge_invalid code => principal_knowledge_invalid (\"Attempting to remove constant_meta from invalid principal_knowledge; \" ++ code)",
		"\t| principal_knowledge_c cm next => match eqb name \"\" with",
		"\t\t\t\t\t\t\t\t\t| true => principal_knowledge_invalid \"Attempting to remove a constant_meta with an empty string as its name\"",
		"\t\t\t\t\t\t\t\t\t| false => match eqb name (get_name_constant_meta cm) with",
		"\t\t\t\t\t\t\t\t\t\t\t| true => next",
		"\t\t\t\t\t\t\t\t\t\t\t| false => principal_knowledge_constructor cm (remove_constant_meta_pk next name)",
		"\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint update_constant_meta_pk (pk: principal_knowledge) (cm: constant_meta): principal_knowledge :=",
		"\tmatch pk with",
		"\t| principal_knowledge_invalid code => principal_knowledge_invalid (\"Attempting to update a constant_meta in an invalid principal_knowledge; \" ++ code)",
		"\t| principal_knowledge_empty => principal_knowledge_invalid \"constant_meta not found\"",
		"\t| principal_knowledge_c _ _ => match cm with",
		"\t\t\t\t\t\t\t\t| constant_meta_invalid _ => principal_knowledge_invalid \"Attempting to update a constant_meta using an invalid principal\"",
		"\t\t\t\t\t\t\t\t| constant_meta_c _ _ _ _ _ _ => principal_knowledge_constructor cm (remove_constant_meta_pk pk (get_name_constant_meta cm))",
		"\t\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint leak_constant_meta_pk (pk: principal_knowledge) (name: string) : principal_knowledge :=",
		"\tmatch pk with",
		"\t| principal_knowledge_invalid code => principal_knowledge_invalid (\"Attempting to leak constant_meta in invalid principal_knowledge; \" ++ code)",
		"\t| principal_knowledge_empty => principal_knowledge_invalid \"Attempting to leak constant_meta in empty principal_knowledge\"",
		"\t| principal_knowledge_c _ _ => update_constant_meta_pk pk (leak_constant_meta(get_constant_meta_by_name_pk pk name))",
		"\tend.",
		"",
		"Inductive principal : Type :=",
		"\t| principal_invalid (code: string)",
		"\t| principal_c (name: string) (pk: principal_knowledge).",
		"",
		"Fixpoint principal_constructor (name: string) (pk: principal_knowledge) : principal :=",
		"\tmatch eqb name \"\" with",
		"\t| true => principal_invalid \"Attempt to construct a principal without a name.\"",
		"\t| false => principal_c name pk",
		"\tend.",
		"",
		"Fixpoint teach_principal (p: principal) (cm: constant_meta) : principal :=",
		"\tmatch p with",
		"\t| principal_invalid _ => p",
		"\t| principal_c name knowledge => principal_constructor name (push_pk knowledge cm)",
		"\tend.",
		"",
		"Fixpoint generate_value (p: principal) (s: string) : principal :=",
		"\tmatch eqb \"\" s with",
		"\t| true => principal_invalid \"Generated value must have a non empty string as its name.\"",
		"\t| false => match p with",
		"\t\t\t| principal_invalid _ => p",
		"\t\t\t| principal_c name _ => teach_principal p (constant_meta_constructor (value s) generates private name s)",
		"\t\t\tend",
		"\tend.",
		"",
		"Fixpoint know_value (p: principal) (s: string) (q: qualifier) : principal :=",
		"\tmatch eqb \"\" s with",
		"\t| true => principal_invalid \"Value to be known must have a non empty string as its name.\"",
		"\t| false => match p with",
		"\t\t\t| principal_invalid _ => p",
		"\t\t\t| principal_c name _ => teach_principal p (constant_meta_constructor (value s) knows q name s)",
		"\t\t\tend",
		"\tend.",
		"",
		"Fixpoint assign_value (p: principal) (c: constant) (s: string) : principal :=",
		"\tmatch eqb \"\" s with",
		"\t| true => principal_invalid \"Assigned value must have a non empty string as its name.\"",
		"\t| false => match p with",
		"\t\t\t| principal_invalid code => p",
		"\t\t\t| principal_c name _ => teach_principal p (constant_meta_constructor c assignment private name s)",
		"\t\t\tend",
		"\tend.",
		"",
		"Fixpoint get_name_principal (p: principal) : string :=",
		"\tmatch p with",
		"\t| principal_invalid code => code",
		"\t| principal_c name _ => name",
		"\tend.",
		"",
		"Fixpoint get_constant_meta_by_name_principal (p: principal) (name: string) : constant_meta :=",
		"\tmatch eqb \"\" name with",
		"\t| true => constant_meta_invalid \"Attempting to look for a value with an empty string as its name\"",
		"\t| false => match p with",
		"\t\t| principal_invalid _ => constant_meta_invalid \"Value not found.\"",
		"\t\t| principal_c _ k => get_constant_meta_by_name_pk k name",
		"\t\tend",
		"\tend.",
		"",
		"Fixpoint leak_value (p: principal) (value_name: string) : principal :=",
		"\tmatch eqb \"\" value_name with",
		"\t| true => principal_invalid \"Attepmting to leak a value with an invalid name.\"",
		"\t| false => match p with",
		"\t\t\t\t| principal_invalid code => principal_invalid (\"Attempting to leak a value in an invalid principal; \" ++ code)",
		"\t\t\t\t| principal_c principal_name pk => principal_constructor principal_name (leak_constant_meta_pk pk value_name)",
		"\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint get (p: principal) (name: string) : constant :=",
		"\tmatch (get_constant_meta_by_name_principal p name) with",
		"\t| constant_meta_invalid code => INVALID code",
		"\t| constant_meta_c c' _ _ _ _ _ => c'",
		"\tend.",
		"",
		"Inductive principal_list : Type :=",
		"\t| principal_list_invalid (code: string)",
		"\t| principal_list_empty",
		"\t| principal_list_c (p: principal) (next: principal_list).",
		"Fixpoint principal_list_constructor (p: principal) (next: principal_list) : principal_list :=",
		"\tmatch p with",
		"\t| principal_invalid code => principal_list_invalid (\"Cannot construct principal_list using invalid principal; \" ++ code)",
		"\t| principal_c _ _ => match next with ",
		"\t\t\t\t\t\t| principal_list_invalid code => principal_list_invalid (\"Cannot construct principal_list using invalid next principal_list; \" ++ code)",
		"\t\t\t\t\t\t| _ => principal_list_c p next",
		"\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint add_principal (list: principal_list) (p: principal) : principal_list :=",
		"\tmatch list with",
		"\t| principal_list_invalid code => principal_list_invalid (\"Cannot add principal to invalid list; \" ++ code)",
		"\t| principal_list_empty => principal_list_constructor p list",
		"\t| principal_list_c _ next => principal_list_constructor p list",
		"\tend.",
		"",
		"Fixpoint remove_principal (list: principal_list) (name: string) : principal_list :=",
		"\tmatch list with",
		"| principal_list_invalid code => principal_list_invalid (\"Attempting to remove a principal from an invalid principal_list; \" ++ code)",
		"| principal_list_empty => principal_list_invalid \"Principal not found\"",
		"| principal_list_c p next => match eqb name \"\" with",
		"\t\t\t\t\t\t\t| true => principal_list_invalid \"Attempting to remove a principal with an empty string as its name\"",
		"\t\t\t\t\t\t\t| false => match eqb name (get_name_principal p) with",
		"\t\t\t\t\t\t\t\t\t| true => next",
		"\t\t\t\t\t\t\t\t\t| false => principal_list_constructor p (remove_principal next name)",
		"\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\tend",
		"\tend.",
		"Fixpoint update_principal (list: principal_list) (p: principal): principal_list :=",
		"\tmatch list with",
		"\t| principal_list_invalid code => principal_list_invalid (\"Attempting to update a principal in an invalid principal_list; \" ++ code)",
		"\t| principal_list_empty => principal_list_invalid \"Principal not found\"",
		"\t| principal_list_c _ _ => match p with",
		"\t\t\t\t\t\t\t| principal_invalid _ => principal_list_invalid \"Attempting to update a principal_list using an invalid principal\"",
		"\t\t\t\t\t\t\t| principal_c _ _ => principal_list_constructor p (remove_principal list (get_name_principal p))",
		"\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint get_principal_by_name_principal_list (list: principal_list) (name: string) : principal :=",
		"\tmatch list with",
		"\t| principal_list_invalid code => principal_invalid (\"Attempting to get a principal from an invalid principal_list; \" ++ code)",
		"\t| principal_list_empty => principal_invalid \"Principal not found\"",
		"\t| principal_list_c p list' => match eqb name \"\" with",
		"\t\t\t\t\t\t\t| true => principal_invalid \"The provided name for the principal cannot be empty\"",
		"\t\t\t\t\t\t\t| false => match eqb (get_name_principal p) name with",
		"\t\t\t\t\t\t\t\t\t\t| true => p",
		"\t\t\t\t\t\t\t\t\t\t| false => get_principal_by_name_principal_list list' name",
		"\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Fixpoint teach_principal_principal_list (list: principal_list) (principal_name: string) (cm: constant_meta) : principal_list :=",
		"\tmatch cm with",
		"\t| constant_meta_invalid code => principal_list_invalid (\"Attempting to teach an invalid constant_meta to a principal; \" ++ code)",
		"\t| constant_meta_c _ _ _ _ _ _ => match eqb principal_name \"\" with",
		"\t\t\t\t\t\t\t\t\t\t| true => principal_list_invalid \"The provided name for the principal cannot be empty\"",
		"\t\t\t\t\t\t\t\t\t\t| false => match list with",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_list_invalid code => principal_list_invalid (\"Attempting to teach a principal in an invalid principal_list; \" ++ code)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_list_empty => add_principal list (teach_principal (principal_constructor principal_name principal_knowledge_empty) cm)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_list_c p list' => update_principal list (teach_principal (get_principal_by_name_principal_list list principal_name) cm)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Inductive message : Type :=",
		"\t| message_c (from to value_name: string) (g: guard_state)",
		"\t| message_invalid (code: string).",
		"Fixpoint message_constructor (from to value_name: string) (g: guard_state) :=",
		"\tmatch eqb \"\" from, eqb \"\" to, eqb \"\" value_name with",
		"\t| true, _, _ => message_invalid \"The value of from cannot be empty\"",
		"\t| _, true, _ => message_invalid \"The value of to cannot be empty\"",
		"\t| _, _, true => message_invalid \"The value of value_name cannot be empty\"",
		"\t| false, false, false => message_c from to value_name g",
		"\tend.",
		"",
		"Inductive message_list : Type :=",
		"\t| message_list_invalid (code: string)",
		"\t| message_list_empty",
		"\t| message_list_c (m: message) (next: message_list).",
		"Fixpoint message_list_constructor (m: message) : message_list :=",
		"\tmatch m with",
		"\t| message_invalid _ => message_list_invalid \"Attempting to construct message_list using an invalid message\"",
		"\t| message_c _ _ _ _ => message_list_c m message_list_empty",
		"\tend.",
		"",
		"Fixpoint add_message_to_list (list: message_list) (m: message) : message_list :=",
		"\tmatch m with",
		"\t| message_invalid _ => message_list_invalid \"Attempting to add invalid message to list\"",
		"\t| message_c _ _ _ _ => match list with",
		"\t\t\t\t\t\t\t| message_list_invalid _ => message_list_invalid \"Attempting to add message to invalid message_list\"",
		"\t\t\t\t\t\t\t| message_list_empty => message_list_constructor m",
		"\t\t\t\t\t\t\t| message_list_c _ next => add_message_to_list next m",
		"\t\t\t\t\t\t\tend",
		"\tend.",
		"",
		"Inductive knowledgemap : Type :=",
		"\t| knowledgemap_invalid (code: string)",
		"\t| knowledgemap_c (list: principal_list) (messages: message_list).",
		"Fixpoint knowledgemap_constructor (principal_name: string) : knowledgemap :=",
		"\tmatch eqb principal_name \"\" with",
		"\t| true => knowledgemap_invalid \"Attempting to construct knowledge map with empty principal name\"",
		"\t| false => knowledgemap_c (principal_list_constructor (principal_constructor principal_name principal_knowledge_empty) principal_list_empty) message_list_empty",
		"\tend.",
		"",
		"Fixpoint add_principal_knowledgemap (k: knowledgemap) (name: string) : knowledgemap :=",
		"\tmatch k with",
		"\t| knowledgemap_invalid code => knowledgemap_invalid (\"Attempting to add principal to invalid knowledgemap; \" ++ code)",
		"\t| knowledgemap_c list m => knowledgemap_c (add_principal list (principal_constructor name principal_knowledge_empty)) m",
		"\tend.",
		"",
		"Fixpoint get_principal_knowledgemap (k: knowledgemap) (name: string) : principal :=",
		"\tmatch k with",
		"\t| knowledgemap_invalid code => principal_invalid (\"Attempting to get principal from invalid knowledgemap; \" ++ code)",
		"\t| knowledgemap_c list _ => get_principal_by_name_principal_list list name",
		"\tend.",
		"",
		"Fixpoint update_principal_knowledgemap (k: knowledgemap) (p: principal) : knowledgemap :=",
		"\tmatch k with",
		"\t| knowledgemap_invalid code => knowledgemap_invalid (\"Attempting to update principal in invalid knowledgemap; \" ++ code)",
		"\t| knowledgemap_c list m => knowledgemap_c (update_principal list p) m",
		"\tend.",
		"",
		"Fixpoint add_message_knowledgemap (k: knowledgemap) (m: message) : knowledgemap :=",
		"\tmatch k with",
		"\t| knowledgemap_invalid code => knowledgemap_invalid (\"Attempting to add message to invalid knowledgemap; \" ++ code)",
		"\t| knowledgemap_c list messages => knowledgemap_c list (add_message_to_list messages m)",
		"\tend.",
		"",
		"Fixpoint send_message (s: knowledgemap): knowledgemap :=",
		"\tmatch s with",
		"\t| knowledgemap_invalid _ => knowledgemap_invalid \"Attempting to send a message using an invalid knowledgemap\"",
		"\t| knowledgemap_c list messages => match messages with",
		"\t\t\t\t\t\t\t| message_list_invalid _ => knowledgemap_invalid \"Invalid message list\"",
		"\t\t\t\t\t\t\t| message_list_empty => s",
		"\t\t\t\t\t\t\t| message_list_c m next => match m with ",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t| message_invalid _ => knowledgemap_invalid \"Attempting to send an invalid message\"",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t| message_c from to value_name g => match get_principal_by_name_principal_list list from with",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_invalid code => knowledgemap_invalid (\"The sender provided is not valid; \" ++ code)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_c _ sender_pk => match get_constant_meta_by_name_pk sender_pk value_name with",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| constant_meta_invalid code => knowledgemap_invalid (\"The sender does now list know the value being sent; \" ++ code)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| constant_meta_c _ _ _ _ _ _ => match get_principal_by_name_principal_list list to with",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_invalid code => knowledgemap_invalid (\"The recipient provided is not valid; \" ++ code)",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t| principal_c _ recipient_pk => knowledgemap_c (teach_principal_principal_list list to (get_constant_meta_by_name_pk sender_pk value_name)) next",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\t\t\tend",
		"\t\t\t\t\t\t\tend",
		"\tend.",
		"",
	}, "\n")
}