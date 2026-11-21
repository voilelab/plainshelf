import Formal.Json

namespace SourceOffsets

structure Line where
  len : Nat
  brk : Nat

def docToSource : List Line → Nat → Nat
  | [], p => p
  | [_], p => p
  | l :: m :: rest, p =>
    if p ≤ l.len then p else (l.len + l.brk) + docToSource (m :: rest) (p - l.len - 1)

def sourceToDoc : List Line → Nat → Nat
  | [], o => o
  | [_], o => o
  | l :: m :: rest, o =>
    if o < l.len + l.brk then min o l.len
    else (l.len + 1) + sourceToDoc (m :: rest) (o - l.len - l.brk)

def docLength : List Line → Nat
  | [] => 0
  | [l] => l.len
  | l :: m :: rest => l.len + 1 + docLength (m :: rest)

def sourceLength : List Line → Nat
  | [] => 0
  | [l] => l.len
  | l :: m :: rest => l.len + l.brk + sourceLength (m :: rest)

def withBreak (w : Nat) (lines : List Line) : List Line :=
  lines.map fun l => { l with brk := w }

def crlfDocToSource (lines : List Line) (p : Nat) : Nat := docToSource (withBreak 2 lines) p
def crlfSourceToDoc (lines : List Line) (o : Nat) : Nat := sourceToDoc (withBreak 2 lines) o

def breaksAre (w : Nat) : List Line → Prop
  | [] => True
  | [_] => True
  | l :: m :: rest => l.brk = w ∧ breaksAre w (m :: rest)

def editorUsesCrlf : List Line → Bool
  | [] => false
  | [_] => false
  | l :: m :: rest => l.brk == 2 && (match rest with
    | [] => true
    | _ => editorUsesCrlf (m :: rest))

def hasWideBreak : List Line → Bool
  | [] => false
  | [_] => false
  | l :: m :: rest => l.brk == 2 || hasWideBreak (m :: rest)

def validBreaks : List Line → Prop
  | [] => True
  | [_] => True
  | l :: m :: rest => (l.brk = 1 ∨ l.brk = 2) ∧ validBreaks (m :: rest)

def identityDocToSource (lines : List Line) (p : Nat) : Nat := min p (docLength lines)
def identitySourceToDoc (lines : List Line) (o : Nat) : Nat := min o (docLength lines)

def editorDocToSource (lines : List Line) (p : Nat) : Nat :=
  if editorUsesCrlf lines then crlfDocToSource lines p
  else if hasWideBreak lines then docToSource lines p
  else identityDocToSource lines p

def editorSourceToDoc (lines : List Line) (o : Nat) : Nat :=
  if editorUsesCrlf lines then crlfSourceToDoc lines o
  else if hasWideBreak lines then sourceToDoc lines o
  else identitySourceToDoc lines o

theorem roundtrip_doc (lines : List Line) (p : Nat)
    (hbrk : ∀ l ∈ lines, 1 ≤ l.brk) :
    sourceToDoc lines (docToSource lines p) = p := by
  induction lines generalizing p with
  | nil => rfl
  | cons l rest ih =>
    cases rest with
    | nil => rfl
    | cons m rest =>
      have hl : 1 ≤ l.brk := hbrk l (by simp)
      have hrest : ∀ x ∈ m :: rest, 1 ≤ x.brk := fun x hx => hbrk x (by simp [hx])
      by_cases hp : p ≤ l.len
      · simp only [docToSource, hp, ite_true, sourceToDoc]
        rw [ite_eq_left_of_eq_true _ _ (eq_true (by omega))]
        omega
      · simp only [docToSource, hp, ite_false, sourceToDoc]
        rw [ite_eq_right_of_eq_false _ _ (eq_false (by omega))]
        have : l.len + l.brk + docToSource (m :: rest) (p - l.len - 1) - l.len - l.brk =
            docToSource (m :: rest) (p - l.len - 1) := by omega
        rw [this, ih _ hrest]
        omega

theorem roundtrip_source_lower (lines : List Line) (o : Nat) :
    docToSource lines (sourceToDoc lines o) ≤ o := by
  induction lines generalizing o with
  | nil => simp [docToSource, sourceToDoc]
  | cons l rest ih =>
    cases rest with
    | nil => simp [docToSource, sourceToDoc]
    | cons m rest =>
      by_cases ho : o < l.len + l.brk
      · simp only [sourceToDoc, ho, ite_true, docToSource]
        rw [ite_eq_left_of_eq_true _ _ (eq_true (by omega))]
        omega
      · simp only [sourceToDoc, ho, ite_false, docToSource]
        rw [ite_eq_right_of_eq_false _ _ (eq_false (by omega))]
        have h := ih (o - l.len - l.brk)
        have : l.len + 1 + sourceToDoc (m :: rest) (o - l.len - l.brk) - l.len - 1 =
            sourceToDoc (m :: rest) (o - l.len - l.brk) := by omega
        rw [this]
        omega

theorem docToSource_mono (lines : List Line) (p q : Nat) (hpq : p ≤ q)
    (hbrk : ∀ l ∈ lines, 1 ≤ l.brk) :
    docToSource lines p ≤ docToSource lines q := by
  induction lines generalizing p q with
  | nil => simpa [docToSource]
  | cons l rest ih =>
    cases rest with
    | nil => simpa [docToSource]
    | cons m rest =>
      have hl : 1 ≤ l.brk := hbrk l (by simp)
      have hrest : ∀ x ∈ m :: rest, 1 ≤ x.brk := fun x hx => hbrk x (by simp [hx])
      by_cases hp : p ≤ l.len <;> by_cases hq : q ≤ l.len <;>
        simp only [docToSource, hp, hq, ite_true, ite_false]
      · omega
      · omega
      · omega
      · have := ih (p - l.len - 1) (q - l.len - 1) (by omega) hrest
        omega

theorem withBreak_breaksAre (w : Nat) (lines : List Line) : breaksAre w (withBreak w lines) := by
  induction lines with
  | nil => trivial
  | cons l rest ih =>
    cases rest with
    | nil => trivial
    | cons m rest => exact ⟨rfl, ih⟩

theorem withBreak_same (w : Nat) (lines : List Line) (h : breaksAre w lines) :
    ∀ p, docToSource (withBreak w lines) p = docToSource lines p ∧
      sourceToDoc (withBreak w lines) p = sourceToDoc lines p := by
  induction lines with
  | nil => intro p; exact ⟨rfl, rfl⟩
  | cons l rest ih =>
    cases rest with
    | nil => intro p; exact ⟨rfl, rfl⟩
    | cons m rest =>
      obtain ⟨hl, hrest⟩ := h
      intro p
      have ihp := ih hrest
      simp only [withBreak, List.map_cons] at ihp ⊢
      constructor
      · simp only [docToSource, hl, (ihp _).1]
      · simp only [sourceToDoc, hl, (ihp _).2]

theorem editorUsesCrlf_breaks (lines : List Line) (h : editorUsesCrlf lines = true) :
    breaksAre 2 lines := by
  induction lines with
  | nil => simp [editorUsesCrlf] at h
  | cons l rest ih =>
    cases rest with
    | nil => simp [editorUsesCrlf] at h
    | cons m rest =>
      simp only [editorUsesCrlf, Bool.and_eq_true, beq_iff_eq] at h
      refine ⟨h.1, ?_⟩
      cases rest with
      | nil => trivial
      | cons n rest => exact ih h.2

theorem crlf_editor_matches (lines : List Line) (h : editorUsesCrlf lines = true) (p : Nat) :
    editorDocToSource lines p = docToSource lines p ∧
      editorSourceToDoc lines p = sourceToDoc lines p := by
  have hb := withBreak_same 2 lines (editorUsesCrlf_breaks lines h) p
  simp only [editorDocToSource, editorSourceToDoc, h, ite_true, crlfDocToSource,
    crlfSourceToDoc]
  exact hb

theorem narrow_breaks (lines : List Line) (h : validBreaks lines)
    (hw : hasWideBreak lines = false) : breaksAre 1 lines := by
  induction lines with
  | nil => trivial
  | cons l rest ih =>
    cases rest with
    | nil => trivial
    | cons m rest =>
      simp only [hasWideBreak, Bool.or_eq_false_iff, beq_eq_false_iff_ne] at hw
      obtain ⟨hl, hrest⟩ := h
      exact ⟨by omega, ih hrest hw.2⟩

theorem narrow_lengths (lines : List Line) (h : breaksAre 1 lines) :
    sourceLength lines = docLength lines := by
  induction lines with
  | nil => rfl
  | cons l rest ih =>
    cases rest with
    | nil => rfl
    | cons m rest =>
      obtain ⟨hl, hrest⟩ := h
      simp only [sourceLength, docLength, hl, ih hrest]

theorem lf_identity (lines : List Line) (h : breaksAre 1 lines) (p : Nat)
    (hp : p ≤ docLength lines) :
    docToSource lines p = p ∧ sourceToDoc lines p = p := by
  induction lines generalizing p with
  | nil => exact ⟨rfl, rfl⟩
  | cons l rest ih =>
    cases rest with
    | nil => exact ⟨rfl, rfl⟩
    | cons m rest =>
      obtain ⟨hl, hrest⟩ := h
      simp only [docLength] at hp
      by_cases hle : p ≤ l.len
      · simp only [docToSource, sourceToDoc, hle, ite_true, hl, true_and]
        split <;> omega
      · have := ih hrest (p - l.len - 1) (by omega)
        simp only [docToSource, sourceToDoc, hle, ite_false, hl]
        split <;> constructor <;> omega

theorem editor_matches (lines : List Line) (hvalid : validBreaks lines) :
    (∀ p, p ≤ docLength lines → editorDocToSource lines p = docToSource lines p) ∧
      (∀ o, o ≤ sourceLength lines → editorSourceToDoc lines o = sourceToDoc lines o) := by
  by_cases hc : editorUsesCrlf lines = true
  · exact ⟨fun p _ => (crlf_editor_matches lines hc p).1,
      fun o _ => (crlf_editor_matches lines hc o).2⟩
  · by_cases hw : hasWideBreak lines = true
    · simp [editorDocToSource, editorSourceToDoc, hc, hw]
    · have hn := narrow_breaks lines hvalid (by simpa using hw)
      have hlen := narrow_lengths lines hn
      refine ⟨fun p hp => ?_, fun o ho => ?_⟩
      · simp only [editorDocToSource, hc, hw, Bool.false_eq_true, ite_false,
          identityDocToSource, (lf_identity lines hn p hp).1]
        omega
      · rw [hlen] at ho
        simp only [editorSourceToDoc, hc, hw, Bool.false_eq_true, ite_false,
          identitySourceToDoc, (lf_identity lines hn o ho).2]
        omega

def mixedSample : List Line := [⟨5, 2⟩, ⟨5, 1⟩, ⟨7, 0⟩]

theorem identity_drifts_on_mixed :
    editorUsesCrlf mixedSample = false ∧
      docToSource mixedSample 6 = 7 ∧ identityDocToSource mixedSample 6 = 6 ∧
      sourceToDoc mixedSample 7 = 6 ∧ identitySourceToDoc mixedSample 7 = 7 := by
  decide

def vectorLayouts : List (List Line) :=
  [[⟨3, 2⟩, ⟨0, 2⟩, ⟨5, 2⟩, ⟨2, 0⟩],
   [⟨0, 2⟩, ⟨4, 2⟩, ⟨0, 0⟩],
   [⟨6, 0⟩],
   [⟨3, 1⟩, ⟨0, 1⟩, ⟨5, 1⟩, ⟨2, 0⟩],
   [⟨0, 1⟩, ⟨4, 1⟩, ⟨0, 0⟩],
   [⟨5, 2⟩, ⟨5, 1⟩, ⟨7, 0⟩],
   [⟨0, 1⟩, ⟨2, 2⟩, ⟨0, 2⟩, ⟨3, 1⟩, ⟨1, 0⟩]]

def vectorLine (lines : List Line) : String :=
  let layout := "[" ++ ",".intercalate (lines.map fun l => s!"[{l.len},{l.brk}]") ++ "]"
  let doc := (List.range (docLength lines + 1)).map (editorDocToSource lines)
  let src := (List.range (sourceLength lines + 1)).map (editorSourceToDoc lines)
  s!"\{\"lines\":{layout},\"docToSource\":{Formal.natArray doc},\"sourceToDoc\":{Formal.natArray src}}"

def vectorsJson : String := Formal.jsonLines (vectorLayouts.map vectorLine)

end SourceOffsets
