      *================================================================
      * ERRREG - AREA DE ERROR QUE SE PASA A LA RUTINA ERRORES
      *================================================================
       01  ERR-AREA.
           05  ERR-PROGRAMA        PIC X(8).
           05  ERR-CODIGO          PIC 9(4).
           05  ERR-TEXTO           PIC X(40).
           05  ERR-SEVERIDAD       PIC 9.
               88  ERR-AVISO       VALUE 1.
               88  ERR-GRAVE       VALUE 2.
               88  ERR-FATAL       VALUE 3.
           05  ERR-RETORNO         PIC 99.
