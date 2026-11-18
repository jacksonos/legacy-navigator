      *================================================================
      * FECHAREG - AREA DE COMUNICACION CON LA RUTINA DE FECHAS
      * FUNCIONES: V VALIDA, D DIFERENCIA EN DIAS, S SUMA DIAS,
      *            H DIA HABIL, F ULTIMO DIA DEL MES
      *================================================================
       01  FEC-AREA.
           05  FEC-FUNCION         PIC X.
           05  FEC-FECHA-1         PIC 9(8).
           05  FEC-FECHA-2         PIC 9(8).
           05  FEC-DIAS            PIC S9(5).
           05  FEC-RESULTADO       PIC 9(8).
           05  FEC-RC              PIC 99.
               88  FEC-OK          VALUE 0.
               88  FEC-INVALIDA    VALUE 1.
