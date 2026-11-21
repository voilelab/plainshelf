namespace ReaderProgress

def roundDiv (a b : Nat) : Nat := (2 * a + b) / (2 * b)

def locate : List Nat → Nat → Nat × Nat
  | [], y => (0, y)
  | x :: xs, y => if y < x then (0, y) else ((locate xs (y - x)).1 + 1, (locate xs (y - x)).2)

def place : List Nat → Nat → Nat → Nat
  | [], _, r => r
  | _ :: _, 0, r => r
  | x :: xs, i + 1, r => x + place xs i r

def blockAt (heights : List Nat) (y : Nat) : Nat := (locate heights y).1

def maxScroll (heights : List Nat) (viewport : Nat) : Nat := heights.sum - viewport

def saveLinear (chars heights : List Nat) (viewport scrollTop : Nat) : Nat :=
  if maxScroll heights viewport = 0 then 0
  else roundDiv (chars.sum * scrollTop) (maxScroll heights viewport)

def restoreLinear (chars heights : List Nat) (viewport offset : Nat) : Nat :=
  if chars.sum = 0 then 0
  else roundDiv (maxScroll heights viewport * offset) chars.sum

def savePiecewise (chars heights : List Nat) (scrollTop : Nat) : Nat :=
  let (i, r) := locate heights scrollTop
  place chars i (r * chars.getD i 0 / heights.getD i 1)

def restorePiecewise (chars heights : List Nat) (offset : Nat) : Nat :=
  let (j, q) := locate chars offset
  place heights j (q * heights.getD j 0 / chars.getD j 1)

theorem locate_place (xs : List Nat) (i r : Nat) (hi : i < xs.length) (hr : r < xs.getD i 0) :
    locate xs (place xs i r) = (i, r) := by
  induction xs generalizing i with
  | nil => simp at hi
  | cons x xs ih =>
    cases i with
    | zero =>
      simp only [List.getD_cons_zero] at hr
      simp [place, locate, hr]
    | succ i =>
      simp only [List.length_cons] at hi
      simp only [List.getD_cons_succ] at hr
      have h := ih i (by omega) hr
      simp [place, locate, h]

theorem locate_bounds (xs : List Nat) (y : Nat) (hy : y < xs.sum) :
    (locate xs y).1 < xs.length ∧ (locate xs y).2 < xs.getD (locate xs y).1 0 := by
  induction xs generalizing y with
  | nil => simp at hy
  | cons x xs ih =>
    simp only [List.sum_cons] at hy
    by_cases h : y < x
    · simp [locate, h]
    · have := ih (y - x) (by omega)
      simp only [locate, h, ite_false, List.length_cons, List.getD_cons_succ]
      omega

theorem scaled_lt (r c h : Nat) (hr : r < h) (hc : 0 < c) : r * c / h < c := by
  rw [Nat.div_lt_iff_lt_mul (by omega)]
  rw [Nat.mul_comm c h]
  exact Nat.mul_lt_mul_of_pos_right hr hc

theorem getD_pos (xs : List Nat) (i : Nat) (hi : i < xs.length) (hpos : ∀ x ∈ xs, 0 < x) :
    0 < xs.getD i 0 := by
  simp only [List.getD, List.getElem?_eq_getElem hi, Option.getD_some]
  exact hpos _ (List.getElem_mem hi)

theorem piecewise_keeps_block (chars saved restored : List Nat) (scrollTop : Nat)
    (hsaved : saved.length = chars.length) (hrestored : restored.length = chars.length)
    (hchars : ∀ c ∈ chars, 0 < c) (hpos : ∀ h ∈ restored, 0 < h)
    (htop : scrollTop < saved.sum) :
    blockAt restored (restorePiecewise chars restored (savePiecewise chars saved scrollTop)) =
      blockAt saved scrollTop := by
  obtain ⟨hi, hr⟩ := locate_bounds saved scrollTop htop
  generalize hloc : locate saved scrollTop = loc at hi hr
  obtain ⟨i, r⟩ := loc
  simp only at hi hr
  have hic : i < chars.length := by omega
  have hc : 0 < chars.getD i 0 := getD_pos chars i hic hchars
  have hsaved_i : saved.getD i 1 = saved.getD i 0 := by
    simp [List.getD, hi]
  have hq : r * chars.getD i 0 / saved.getD i 1 < chars.getD i 0 := by
    rw [hsaved_i]; exact scaled_lt _ _ _ hr hc
  have hchars_loc := locate_place chars i _ hic hq
  have hir : i < restored.length := by omega
  have hh : 0 < restored.getD i 0 := getD_pos restored i hir hpos
  have hchars_i : chars.getD i 1 = chars.getD i 0 := by
    simp [List.getD, hic]
  have hq' : (r * chars.getD i 0 / saved.getD i 1) * restored.getD i 0 / chars.getD i 1
      < restored.getD i 0 := by
    rw [hchars_i]; exact scaled_lt _ _ _ hq hh
  have hrestored_loc := locate_place restored i _ hir hq'
  unfold blockAt savePiecewise restorePiecewise
  rw [hloc]
  dsimp only
  rw [hchars_loc]
  dsimp only
  rw [hrestored_loc]

def sampleChars : List Nat := [400, 400, 400, 400, 400, 400]
def sampleMeasured : List Nat := [400, 400, 2000, 400, 400, 400]
def sampleEstimated : List Nat := [400, 400, 320, 400, 400, 400]
def sampleViewport : Nat := 800
def sampleScrollTop : Nat := 400

theorem linear_restore_moves_block :
    blockAt sampleEstimated
        (restoreLinear sampleChars sampleEstimated sampleViewport
          (saveLinear sampleChars sampleMeasured sampleViewport sampleScrollTop)) = 0 ∧
      blockAt sampleMeasured sampleScrollTop = 1 := by
  decide

theorem linear_offset_skips_block :
    (locate sampleChars
        (saveLinear sampleChars sampleMeasured sampleViewport sampleScrollTop)).1 = 0 := by
  decide

end ReaderProgress
