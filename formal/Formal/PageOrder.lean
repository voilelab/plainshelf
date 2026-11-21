import Formal.Json

namespace PageOrder

def movePage (l : List α) (src dst : Nat) : List α :=
  if h : src < l.length ∧ dst < l.length ∧ src ≠ dst then
    (l.eraseIdx src).insertIdx dst l[src]
  else l

def dropDestination (src target : Nat) (after : Bool) : Nat :=
  if after then (if target + 1 > src then target else target + 1)
  else (if target > src then target - 1 else target)

theorem perm_eraseIdx (l : List α) (i : Nat) (h : i < l.length) :
    (l[i] :: l.eraseIdx i).Perm l := by
  have hsplit : l = l.take i ++ l[i] :: l.drop (i + 1) := by
    rw [← List.drop_eq_getElem_cons h, List.take_append_drop]
  rw [List.eraseIdx_eq_take_drop_succ]
  conv => rhs; rw [hsplit]
  exact List.perm_middle.symm

theorem movePage_perm (l : List α) (src dst : Nat) :
    (movePage l src dst).Perm l := by
  unfold movePage
  split
  · next h =>
    have hlen := @List.length_eraseIdx _ l src
    exact (List.perm_insertIdx _ _ (by rw [hlen]; split <;> omega)).trans
      (perm_eraseIdx l src h.1)
  · exact List.Perm.refl l

theorem movePage_length (l : List α) (src dst : Nat) :
    (movePage l src dst).length = l.length :=
  (movePage_perm l src dst).length_eq

theorem movePage_keeps_others (l : List α) (src dst : Nat)
    (hs : src < l.length) (hd : dst < l.length) :
    (movePage l src dst).eraseIdx dst = l.eraseIdx src := by
  unfold movePage
  split
  · exact List.eraseIdx_insertIdx_self _
  · next h =>
    have : src = dst := by omega
    subst this
    rfl

theorem dropDestination_lt (src target : Nat) (after : Bool) (n : Nat)
    (hs : src < n) (ht : target < n) :
    dropDestination src target after < n := by
  unfold dropDestination
  cases after <;> simp only [Bool.false_eq_true, ite_true, ite_false] <;> split <;> omega

theorem movePage_self (l : List α) (s : Nat) : movePage l s s = l := by
  simp [movePage]

theorem getElem?_movePage (l : List α) (src dst k : Nat)
    (hs : src < l.length) (hd : dst < l.length) (hne : src ≠ dst) :
    (movePage l src dst)[k]? =
      if k < dst then (l.eraseIdx src)[k]?
      else if k = dst then l[src]?
      else (l.eraseIdx src)[k - 1]? := by
  have hlen := @List.length_eraseIdx _ l src
  unfold movePage
  split
  · rw [List.getElem?_insertIdx, List.getElem?_eq_getElem hs]
    split
    · rfl
    · split
      · rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hlen]; split <;> omega))]
      · rfl
  · omega

macro "finish_drop" : tactic => `(tactic| (
  simp only [List.getElem?_eraseIdx]
  refine ⟨by simp, ?_⟩
  repeat' split
  all_goals first | rfl | omega | (congr 1; omega) | (exfalso; simp_all)))

theorem drop_lands_beside_target (l : List α) (src target : Nat) (after : Bool)
    (hs : src < l.length) (ht : target < l.length) (hne : src ≠ target) :
    (movePage l src (dropDestination src target after))[dropDestination src target after]?
        = l[src]? ∧
      (movePage l src (dropDestination src target after))[
          if after then dropDestination src target after - 1
          else dropDestination src target after + 1]? = l[target]? := by
  rcases Nat.lt_or_gt_of_ne hne with hlt | hgt
  · cases after
    · by_cases hadj : target = src + 1
      · subst hadj
        simp [dropDestination, movePage_self]
      · have hd : dropDestination src target false = target - 1 := by
          simp [dropDestination]; omega
        rw [hd, getElem?_movePage l src _ _ hs (by omega) (by omega),
          getElem?_movePage l src _ _ hs (by omega) (by omega)]
        finish_drop
    · have hd : dropDestination src target true = target := by
        simp [dropDestination]; omega
      rw [hd, getElem?_movePage l src _ _ hs ht hne, getElem?_movePage l src _ _ hs ht hne]
      finish_drop
  · cases after
    · have hd : dropDestination src target false = target := by
        simp [dropDestination]; omega
      rw [hd, getElem?_movePage l src _ _ hs ht hne, getElem?_movePage l src _ _ hs ht hne]
      finish_drop
    · by_cases hadj : src = target + 1
      · subst hadj
        simp [dropDestination, movePage_self]
      · have hd : dropDestination src target true = target + 1 := by
          simp [dropDestination]; omega
        rw [hd, getElem?_movePage l src _ _ hs (by omega) (by omega),
          getElem?_movePage l src _ _ hs (by omega) (by omega)]
        finish_drop

def vectorLines (maxLen : Nat) : List String := Id.run do
  let mut out : List String := []
  for n in List.range (maxLen + 1) do
    let l := List.range n
    for s in List.range n do
      for t in List.range n do
        for after in [false, true] do
          let d := dropDestination s t after
          out := out ++ [s!"\{\"length\":{n},\"from\":{s},\"target\":{t},\"side\":\"{if after then "after" else "before"}\",\"destination\":{d},\"result\":{Formal.natArray (movePage l s d)}}"]
  return out

def vectorsJson : String := Formal.jsonLines (vectorLines 5)

end PageOrder
