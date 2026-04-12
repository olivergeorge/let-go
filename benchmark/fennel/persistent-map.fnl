;; Build a persistent map with 10000 entries
(local core (require :io.gitlab.andreyorst.cljlib.core))

(core.reduce (fn [m i] (core.assoc m i (* i i)))
             {}
             (core.range 10000))
