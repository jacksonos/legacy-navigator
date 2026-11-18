      *================================================================
      * CLIREG - REGISTRO MAESTRO DE CLIENTES
      *================================================================
       01  CLI-REGISTRO.
           05  CLI-NUMERO          PIC 9(8).
           05  CLI-NOMBRE          PIC X(30).
           05  CLI-DOCUMENTO       PIC X(12).
           05  CLI-SEGMENTO        PIC X.
               88  CLI-MINORISTA   VALUE 'M'.
               88  CLI-PREFERENTE  VALUE 'P'.
               88  CLI-EMPRESA     VALUE 'E'.
           05  CLI-ESTADO          PIC X.
               88  CLI-VIGENTE     VALUE 'V'.
               88  CLI-INACTIVO    VALUE 'I'.
           05  CLI-EMAIL           PIC X(40).
           05  CLI-TELEFONO        PIC X(15).
