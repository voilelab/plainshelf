import Formal.Json

namespace FolderOrder

def keyLt : Option Int → Option Int → Bool
  | some x, some y => decide (x < y)
  | _, _ => false

def midKey : Option Int → Option Int → Int
  | none, r => r.getD 0 - 1024
  | some l, none => l + 1024
  | some l, some r => l / 2 + r / 2

def renumberFrom (n : Nat) : List Nat → List (Nat × Int)
  | [] => []
  | p :: ps => (p, ((n : Int) + 1) * 1024) :: renumberFrom (n + 1) ps

def insertSlot (sibs : List Nat) (src tgt : Nat) (after : Bool) : Nat :=
  (sibs.erase src).idxOf tgt + if after then 1 else 0

def nextOrder (sibs : List Nat) (src tgt : Nat) (after : Bool) : List Nat :=
  (sibs.erase src).take (insertSlot sibs src tgt after) ++
    src :: (sibs.erase src).drop (insertSlot sibs src tgt after)

abbrev Sorted (key : Nat → Option Int) (l : List Nat) : Prop :=
  l.Pairwise (fun a b => keyLt (key a) (key b) = true)

def plan (sibs : List Nat) (key : Nat → Option Int) (src tgt : Nat) (after : Bool) :
    List (Nat × Int) :=
  if src = tgt ∨ src ∉ sibs ∨ tgt ∉ sibs then []
  else if nextOrder sibs src tgt after = sibs then []
  else
    let before := (sibs.erase src).take (insertSlot sibs src tgt after)
    let behind := (sibs.erase src).drop (insertSlot sibs src tgt after)
    let left := before.getLast?.bind key
    let right := behind.head?.bind key
    let k := midKey left right
    if Sorted key sibs ∧ left.all (· < k) ∧ right.all (k < ·) then [(src, k)]
    else renumberFrom 0 (nextOrder sibs src tgt after)

def withKey (key : Nat → Option Int) (src : Nat) (k : Int) (p : Nat) : Option Int :=
  if p = src then some k else key p

theorem keyLt_some {a b : Option Int} (h : keyLt a b = true) :
    ∃ x y, a = some x ∧ b = some y ∧ x < y := by
  cases a <;> cases b <;> simp_all [keyLt]

theorem keyLt_trans {a b c : Option Int} (h1 : keyLt a b = true) (h2 : keyLt b c = true) :
    keyLt a c = true := by
  obtain ⟨x, y, rfl, rfl, hxy⟩ := keyLt_some h1
  obtain ⟨y', z, hy, rfl, hyz⟩ := keyLt_some h2
  cases hy
  simp [keyLt]; omega

theorem sorted_keyed (key : Nat → Option Int) (l : List Nat) (h : Sorted key l)
    (x y : Nat) (hx : x ∈ l) (hy : y ∈ l) (hne : x ≠ y) : ∃ v, key x = some v := by
  unfold Sorted at h
  induction l with
  | nil => simp at hx
  | cons a l ih =>
    rw [List.pairwise_cons] at h
    simp only [List.mem_cons] at hx hy
    rcases hx with rfl | hx
    · rcases hy with rfl | hy
      · exact absurd rfl hne
      · obtain ⟨v, _, hv, _, _⟩ := keyLt_some (h.1 y hy); exact ⟨v, hv⟩
    · rcases hy with rfl | hy
      · obtain ⟨_, v, _, hv, _⟩ := keyLt_some (h.1 x hx); exact ⟨v, hv⟩
      · exact ih h.2 hx hy

theorem insert_between (key : Nat → Option Int) (src : Nat) (k : Int)
    (before behind : List Nat)
    (hsorted : Sorted key (before ++ behind))
    (hfresh : src ∉ before ++ behind)
    (hkeyed : ∀ x ∈ before ++ behind, ∃ v, key x = some v)
    (hleft : ∀ x ∈ before.getLast?, ∀ v, key x = some v → v < k)
    (hright : ∀ x ∈ behind.head?, ∀ v, key x = some v → k < v) :
    Sorted (withKey key src k) (before ++ src :: behind) := by
  have hsame : ∀ x ∈ before ++ behind, withKey key src k x = key x := by
    intro x hx
    have : x ≠ src := fun h => hfresh (h ▸ hx)
    simp [withKey, this]
  have hsrc : withKey key src k src = some k := by simp [withKey]
  unfold Sorted at *
  rw [List.pairwise_append] at hsorted ⊢
  obtain ⟨hb, ha, hcross⟩ := hsorted
  have hbehind_gt : ∀ b ∈ behind, ∀ v, key b = some v → k < v := by
    intro b hb' v hv
    cases behind with
    | nil => simp at hb'
    | cons h t =>
      rw [List.pairwise_cons] at ha
      obtain ⟨vh, hvh⟩ := hkeyed h (by simp)
      have hk : k < vh := hright h (by simp) vh hvh
      simp only [List.mem_cons] at hb'
      rcases hb' with rfl | hb'
      · rw [hvh] at hv; cases hv; exact hk
      · obtain ⟨x, y, hx, hy, hxy⟩ := keyLt_some (ha.1 b hb')
        rw [hvh] at hx; cases hx; rw [hv] at hy; cases hy; omega
  have hbefore_lt : ∀ a ∈ before, ∀ v, key a = some v → v < k := by
    intro a ha' v hv
    rcases List.eq_nil_or_concat before with hnil | ⟨pre, last, hpl⟩
    · subst hnil; simp at ha'
    · rw [List.concat_eq_append] at hpl
      subst hpl
      obtain ⟨vl, hvl⟩ := hkeyed last (by simp)
      have hl : vl < k := hleft last (by simp) vl hvl
      rw [List.pairwise_append] at hb
      simp only [List.mem_append, List.mem_singleton] at ha'
      rcases ha' with ha' | rfl
      · obtain ⟨x, y, hx, hy, hxy⟩ := keyLt_some (hb.2.2 a ha' last (by simp))
        rw [hv] at hx; cases hx; rw [hvl] at hy; cases hy; omega
      · rw [hvl] at hv; cases hv; exact hl
  refine ⟨?_, ?_, ?_⟩
  · refine List.Pairwise.imp_of_mem ?_ hb
    intro a b ha' hb' h
    rw [hsame a (by simp [ha']), hsame b (by simp [hb'])]; exact h
  · rw [List.pairwise_cons]
    refine ⟨?_, ?_⟩
    · intro b hb'
      obtain ⟨v, hv⟩ := hkeyed b (by simp [hb'])
      rw [hsrc, hsame b (by simp [hb']), hv]
      simp [keyLt, hbehind_gt b hb' v hv]
    · refine List.Pairwise.imp_of_mem ?_ ha
      intro a b ha' hb' h
      rw [hsame a (by simp [ha']), hsame b (by simp [hb'])]; exact h
  · intro a ha' b hb'
    obtain ⟨va, hva⟩ := hkeyed a (by simp [ha'])
    rw [hsame a (by simp [ha'])]
    simp only [List.mem_cons] at hb'
    rcases hb' with rfl | hb'
    · rw [hsrc, hva]; simp [keyLt, hbefore_lt a ha' va hva]
    · rw [hsame b (by simp [hb'])]; exact hcross a ha' b hb'

theorem single_write_sorts_next (sibs : List Nat) (key : Nat → Option Int)
    (src tgt : Nat) (after : Bool) (k : Int) (hnodup : sibs.Nodup)
    (hplan : plan sibs key src tgt after = [(src, k)]) :
    Sorted (withKey key src k) (nextOrder sibs src tgt after) := by
  unfold plan at hplan
  split at hplan
  · simp at hplan
  split at hplan
  · simp at hplan
  next hguard hmoved =>
  simp only [not_or, Classical.not_not] at hguard
  obtain ⟨hne, hsrc, htgt⟩ := hguard
  dsimp only at hplan
  split at hplan
  · next hcond =>
    simp only [List.cons.injEq, Prod.mk.injEq, and_true] at hplan
    obtain ⟨hsorted, hl, hr⟩ := hcond
    rw [hplan.2] at hl hr
    have hrest : Sorted key (sibs.erase src) :=
      List.Pairwise.sublist (List.erase_sublist) hsorted
    have hfresh : src ∉ sibs.erase src := List.Nodup.not_mem_erase hnodup
    unfold nextOrder
    apply insert_between
    · rw [List.take_append_drop]; exact hrest
    · rw [List.take_append_drop]; exact hfresh
    · rw [List.take_append_drop]
      intro x hx
      have hxs : x ∈ sibs := List.mem_of_mem_erase hx
      have hxne : x ≠ src := fun h => hfresh (h ▸ hx)
      exact sorted_keyed key sibs hsorted x src hxs hsrc hxne
    · intro x hx v hv
      have := hl
      simp only [Option.all] at this
      rw [Option.mem_def] at hx
      simp only [hx, Option.bind_some, hv] at this
      simpa using this
    · intro x hx v hv
      have := hr
      simp only [Option.all] at this
      rw [Option.mem_def] at hx
      simp only [hx, Option.bind_some, hv] at this
      simpa using this
  · next hcond =>
    exfalso
    cases h : nextOrder sibs src tgt after with
    | nil => simp [nextOrder] at h
    | cons p ps =>
      rw [h] at hplan
      simp only [renumberFrom, List.cons.injEq, Prod.mk.injEq] at hplan
      obtain ⟨_, hnil⟩ := hplan
      cases ps with
      | cons _ _ => simp [renumberFrom] at hnil
      | nil =>
        have hlen := congrArg List.length h
        simp only [nextOrder, List.length_append, List.length_take, List.length_cons,
          List.length_drop, List.length_nil] at hlen
        have hmem := List.length_erase_of_mem hsrc
        have htwo : 2 ≤ sibs.length := by
          have h1 : sibs.erase src ≠ [] := by
            intro he
            have : tgt ∈ sibs.erase src := (List.Nodup.mem_erase_iff hnodup).2 ⟨Ne.symm hne, htgt⟩
            simp [he] at this
          have := List.length_pos_iff.2 h1
          omega
        omega

theorem renumber_sorted (n : Nat) (l : List Nat) :
    (renumberFrom n l).map Prod.fst = l ∧
      ((renumberFrom n l).map Prod.snd).Pairwise (· < ·) ∧
      ∀ x ∈ (renumberFrom n l).map Prod.snd, ((n : Int) + 1) * 1024 ≤ x := by
  induction l generalizing n with
  | nil => simp [renumberFrom]
  | cons p ps ih =>
    obtain ⟨h1, h2, h3⟩ := ih (n + 1)
    simp only [renumberFrom, List.map_cons, h1, List.pairwise_cons, List.mem_cons,
      forall_eq_or_imp, true_and]
    refine ⟨⟨?_, h2⟩, Int.le_refl _, ?_⟩
    · intro x hx; have := h3 x hx; push_cast at this; omega
    · intro x hx; have := h3 x hx; push_cast at this; omega

theorem plan_shape (sibs : List Nat) (key : Nat → Option Int) (src tgt : Nat) (after : Bool) :
    plan sibs key src tgt after = [] ∨
      (∃ k, plan sibs key src tgt after = [(src, k)]) ∨
      plan sibs key src tgt after = renumberFrom 0 (nextOrder sibs src tgt after) := by
  unfold plan
  split
  · exact Or.inl rfl
  split
  · exact Or.inl rfl
  dsimp only
  split
  · exact Or.inr (Or.inl ⟨_, rfl⟩)
  · exact Or.inr (Or.inr rfl)

theorem nextOrder_perm (sibs : List Nat) (src tgt : Nat) (after : Bool) (hsrc : src ∈ sibs) :
    (nextOrder sibs src tgt after).Perm sibs := by
  unfold nextOrder
  refine List.Perm.trans ?_ (List.perm_cons_erase hsrc).symm
  refine List.Perm.trans List.perm_middle ?_
  rw [List.take_append_drop]

theorem nextOrder_beside (sibs : List Nat) (src tgt : Nat) (after : Bool)
    (hnodup : sibs.Nodup) (htgt : tgt ∈ sibs) (hne : src ≠ tgt) :
    if after then
      ((sibs.erase src).take (insertSlot sibs src tgt after)).getLast? = some tgt
    else
      ((sibs.erase src).drop (insertSlot sibs src tgt after)).head? = some tgt := by
  have hmem : tgt ∈ sibs.erase src := (List.Nodup.mem_erase_iff hnodup).2 ⟨Ne.symm hne, htgt⟩
  have hlt := List.idxOf_lt_length_iff.2 hmem
  have hget := List.getElem_idxOf hlt
  unfold insertSlot
  cases after
  · simp only [Bool.false_eq_true, ite_false, Nat.add_zero, List.head?_drop,
      List.getElem?_eq_getElem hlt, hget]
  · simp [List.getLast?_take, List.getElem?_eq_getElem hlt, hget]

def keyJson : Option Int → String
  | some k => toString k
  | none => "null"

def planJson (steps : List (Nat × Int)) : String :=
  "[" ++ ",".intercalate (steps.map fun (p, k) => s!"[\"f{p}\",{k}]") ++ "]"

def keyLayouts : List (List (Option Int)) :=
  [[some 0, some 2048, some 4096, some 6144],
   [some 0, some 2, some 4, some 6],
   [some (-4096), some (-2048), some 0, some 2048],
   [some 0, some 0, some 2048, some 4096],
   [some 0, none, some 2048, some 4096],
   [none, none, none, none],
   [some 2048, some 0, some 4096, some 6144]]

def vectorLines : List String := Id.run do
  let mut out : List String := []
  for layout in keyLayouts do
    for n in [2, 3, 4] do
      let keys := layout.take n
      let sibs := List.range n
      let key := fun p => (keys[p]?).getD none
      for s in sibs do
        for t in sibs do
          for after in [false, true] do
            let side := if after then "after" else "before"
            let keysJson := "[" ++ ",".intercalate (keys.map keyJson) ++ "]"
            out := out ++ [s!"\{\"keys\":{keysJson},\"source\":{s},\"target\":{t},\"side\":\"{side}\",\"plan\":{planJson (plan sibs key s t after)}}"]
  return out

def vectorsJson : String := Formal.jsonLines vectorLines

end FolderOrder
