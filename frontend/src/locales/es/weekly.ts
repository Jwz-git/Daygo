export default {
  title: 'Semanal',
  developmentFixture: 'Datos de ejemplo · solo en desarrollo',
  navigation: {
    label: 'Navegación por semana',
    current: 'Esta semana',
    backendRequired: 'Cambiar de semana no está disponible ahora mismo',
  },
  meta: {
    tracked: '{count} min registrados',
  },
  intro: {
    eyebrow: 'Repaso de la semana',
    title: 'En qué se fue la semana',
    description: 'Tu tiempo registrado, el ritmo de concentración, la distribución diaria y la mezcla de categorías de la semana.',
  },
  overview: {
    eyebrow: 'Resumen',
    title: 'Tiempo y concentración',
    description: 'El tiempo de concentración excluye la distracción y la inactividad; el tiempo registrado excluye los marcadores del sistema.',
    focusAria: 'La concentración representa {value} del tiempo registrado',
  },
  metric: {
    tracked: 'Tiempo registrado',
    focused: 'Tiempo de concentración',
    other: 'Otro tiempo',
    focusRate: 'Proporción de concentración',
  },
  categories: {
    eyebrow: 'Composición',
    title: 'Distribución por categoría',
    count: '{count} categorías',
    distributionAria: 'Proporción del tiempo registrado por categoría esta semana',
    total: 'Total',
  },
  daily: {
    eyebrow: 'Patrón',
    title: 'Cronología diaria',
    hint: 'Una fila por día; cada bloque es un tramo de categoría',
    idleTag: 'inactivo',
    noActivity: 'Sin actividad',
  },
  rhythm: {
    eyebrow: 'Ritmo',
    title: 'Ritmo de actividad semanal',
    focus: 'Concentración',
    idle: 'Inactividad',
    chartAria: 'Minutos de concentración e inactividad por hora del día',
    empty: 'Todavía no hay nada que resumir esta semana',
  },
  insights: {
    title: 'La semana de un vistazo',
    activeDays: 'Días activos',
    daysCount: '{count} días',
    ofSeven: 'de 7 días',
    longestFocus: 'Concentración más larga',
    peakHour: 'Hora punta',
    avgFocus: 'Concentración media diaria',
    perActiveDay: 'por día activo',
    busiestDay: 'Día más ajetreado',
    none: 'Ninguno',
  },
  state: {
    loading: {
      title: 'Cargando esta semana',
      description: 'Un momento.',
    },
    unavailable: {
      title: 'La vista semanal no está disponible ahora mismo',
      description: 'Inténtalo más tarde o reinicia Daygo.',
    },
    failure: {
      title: 'No se pudo cargar esta semana',
      description: 'Tus datos no se ven afectados. Puedes intentarlo de nuevo.',
    },
    empty: {
      title: 'Todavía no hay nada registrado esta semana',
      description: 'Tu repaso semanal aparecerá aquí cuando Daygo haya grabado y organizado algo de actividad.',
    },
  },
  scopeNote: 'Basado en tus tarjetas de actividad diaria.',
}
