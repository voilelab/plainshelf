namespace PageOrder

def removeAt : List α → Nat → List α
  | [], _ => []
  | _ :: xs, 0 => xs
  | x :: xs, n + 1 => x :: removeAt xs n

def insertAt (a : α) : List α → Nat → List α
  | xs, 0 => a :: xs
  | [], _ + 1 => [a]
  | x :: xs, n + 1 => x :: insertAt a xs n

def movePage (l : List α) (src dst : Nat) : List α :=
  if h : src < l.length ∧ dst < l.length ∧ src ≠ dst then
    insertAt l[src] (removeAt l src) dst
  else l

def dropDestination (src target : Nat) (after : Bool) : Nat :=
  if after then (if target + 1 > src then target else target + 1)
  else (if target > src then target - 1 else target)

theorem length_removeAt (l : List α) (s : Nat) (h : s < l.length) :
    (removeAt l s).length = l.length - 1 := by
  induction l generalizing s with
  | nil => simp at h
  | cons x xs ih =>
    cases s with
    | zero => simp [removeAt]
    | succ s =>
      simp only [List.length_cons] at h
      simp only [removeAt, List.length_cons]
      rw [ih s (by omega)]
      omega

theorem length_insertAt (a : α) (xs : List α) (n : Nat) (h : n ≤ xs.length) :
    (insertAt a xs n).length = xs.length + 1 := by
  induction xs generalizing n with
  | nil => cases n <;> simp_all [insertAt]
  | cons x xs ih =>
    cases n with
    | zero => simp [insertAt]
    | succ n =>
      simp only [List.length_cons] at h
      simp only [insertAt, List.length_cons]
      rw [ih n (by omega)]

theorem getElem?_removeAt (l : List α) (s k : Nat) :
    (removeAt l s)[k]? = if k < s then l[k]? else l[k + 1]? := by
  induction l generalizing s k with
  | nil => cases s <;> simp [removeAt]
  | cons x xs ih =>
    cases s with
    | zero => simp [removeAt]
    | succ s =>
      cases k with
      | zero => simp [removeAt]
      | succ k =>
        simp only [removeAt, List.getElem?_cons_succ, ih]
        by_cases hk : k < s
        · simp [hk]
        · simp [hk]

theorem getElem?_insertAt (a : α) (xs : List α) (n k : Nat) (h : n ≤ xs.length) :
    (insertAt a xs n)[k]? =
      if k < n then xs[k]? else if k = n then some a else xs[k - 1]? := by
  induction xs generalizing n k with
  | nil =>
    simp only [List.length_nil, Nat.le_zero] at h
    subst h
    cases k <;> simp [insertAt]
  | cons x xs ih =>
    cases n with
    | zero =>
      cases k with
      | zero => simp [insertAt]
      | succ k => simp [insertAt]
    | succ n =>
      simp only [List.length_cons] at h
      cases k with
      | zero => simp [insertAt]
      | succ k =>
        simp only [insertAt, List.getElem?_cons_succ, ih n k (by omega)]
        by_cases h1 : k < n
        · simp [h1]
        · by_cases h2 : k = n
          · simp [h2]
          · have : k ≠ 0 := by omega
            obtain ⟨j, rfl⟩ : ∃ j, k = j + 1 := ⟨k - 1, by omega⟩
            simp [h1, h2]

theorem removeAt_insertAt (a : α) (xs : List α) (n : Nat) (h : n ≤ xs.length) :
    removeAt (insertAt a xs n) n = xs := by
  induction xs generalizing n with
  | nil =>
    simp only [List.length_nil, Nat.le_zero] at h
    subst h
    simp [insertAt, removeAt]
  | cons x xs ih =>
    cases n with
    | zero => simp [insertAt, removeAt]
    | succ n =>
      simp only [List.length_cons] at h
      simp [insertAt, removeAt, ih n (by omega)]

theorem perm_insertAt (a : α) (xs : List α) (n : Nat) :
    (insertAt a xs n).Perm (a :: xs) := by
  induction xs generalizing n with
  | nil => cases n <;> simp [insertAt]
  | cons x xs ih =>
    cases n with
    | zero => simp [insertAt]
    | succ n =>
      simp only [insertAt]
      exact ((ih n).cons x).trans (List.Perm.swap a x xs)

theorem perm_removeAt (l : List α) (s : Nat) (h : s < l.length) :
    (l[s] :: removeAt l s).Perm l := by
  induction l generalizing s with
  | nil => simp at h
  | cons x xs ih =>
    cases s with
    | zero => simp [removeAt]
    | succ s =>
      simp only [List.length_cons] at h
      simp only [removeAt, List.getElem_cons_succ]
      exact (List.Perm.swap x xs[s] (removeAt xs s)).trans
        ((ih s (by omega)).cons x)

theorem movePage_perm (l : List α) (src dst : Nat) :
    (movePage l src dst).Perm l := by
  unfold movePage
  split
  · next h => exact (perm_insertAt _ _ _).trans (perm_removeAt l src h.1)
  · exact List.Perm.refl l

theorem movePage_length (l : List α) (src dst : Nat) :
    (movePage l src dst).length = l.length :=
  (movePage_perm l src dst).length_eq

theorem movePage_keeps_others (l : List α) (src dst : Nat)
    (hs : src < l.length) (hd : dst < l.length) :
    removeAt (movePage l src dst) dst = removeAt l src := by
  unfold movePage
  split
  · next h =>
    exact removeAt_insertAt _ _ _ (by rw [length_removeAt l src hs]; omega)
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
      if k < dst then (removeAt l src)[k]?
      else if k = dst then l[src]?
      else (removeAt l src)[k - 1]? := by
  have hlen := length_removeAt l src hs
  unfold movePage
  split
  · rw [getElem?_insertAt _ _ _ _ (by omega), List.getElem?_eq_getElem hs]
  · omega

macro "finish_drop" : tactic => `(tactic| (
  simp only [getElem?_removeAt]
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
          getElem?_movePage l src _ _ hs (by omega) (by omega),
          ]
        finish_drop
    · have hd : dropDestination src target true = target := by
        simp [dropDestination]; omega
      rw [hd, getElem?_movePage l src _ _ hs ht hne,
        getElem?_movePage l src _ _ hs ht hne, ]
      finish_drop
  · cases after
    · have hd : dropDestination src target false = target := by
        simp [dropDestination]; omega
      rw [hd, getElem?_movePage l src _ _ hs ht hne,
        getElem?_movePage l src _ _ hs ht hne, ]
      finish_drop
    · by_cases hadj : src = target + 1
      · subst hadj
        simp [dropDestination, movePage_self]
      · have hd : dropDestination src target true = target + 1 := by
          simp [dropDestination]; omega
        rw [hd, getElem?_movePage l src _ _ hs (by omega) (by omega),
          getElem?_movePage l src _ _ hs (by omega) (by omega),
          ]
        finish_drop

def jsonList : List Nat → String
  | xs => "[" ++ ",".intercalate (xs.map toString) ++ "]"

def vectorLines (maxLen : Nat) : List String := Id.run do
  let mut out : List String := []
  for n in List.range (maxLen + 1) do
    let l := List.range n
    for s in List.range n do
      for t in List.range n do
        for after in [false, true] do
          let d := dropDestination s t after
          out := out ++ [s!"\{\"length\":{n},\"from\":{s},\"target\":{t},\"side\":\"{if after then "after" else "before"}\",\"destination\":{d},\"result\":{jsonList (movePage l s d)}}"]
  return out

def main : IO Unit := do
  IO.println ("[\n" ++ ",\n".intercalate (vectorLines 5) ++ "\n]")

end PageOrder

def main : IO Unit := PageOrder.main
