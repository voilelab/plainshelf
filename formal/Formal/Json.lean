namespace Formal

def natArray (xs : List Nat) : String := "[" ++ ",".intercalate (xs.map toString) ++ "]"

def jsonLines (items : List String) : String := "[\n" ++ ",\n".intercalate items ++ "\n]\n"

end Formal
