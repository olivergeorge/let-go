(import-macros {: defn} :io.gitlab.andreyorst.cljlib.core)
(local core (require :io.gitlab.andreyorst.cljlib.core))

(defn tak [x y z]
  (if (< y x)
    (tak (tak (core.dec x) y z) (tak (core.dec y) z x) (tak (core.dec z) x y))
    z))

(tak 30 22 12)
