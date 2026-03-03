<template>
  <v-footer class="app-footer">
    <v-container>
      <v-row align="center" justify="center">
        <v-col cols="12" class="text-center">
          <div class="footer-links">
            <RouterLink to="/privacy-policy" class="footer-link">
              Privacy Policy
            </RouterLink>
            <span class="footer-divider">&bull;</span>
            <RouterLink to="/terms-of-service" class="footer-link">
              Terms of Service
            </RouterLink>
          </div>
          <div class="footer-copyright mt-2">
            &copy; {{ currentYear }} Anigraph
          </div>
        </v-col>
      </v-row>
    </v-container>
  </v-footer>
</template>

<script setup lang="ts">
const currentYear = new Date().getFullYear()
</script>

<style scoped>
.app-footer {
  background: linear-gradient(180deg, rgba(var(--color-bg-rgb), 0) 0%, rgba(var(--color-bg-rgb), 0.95) 100%);
  backdrop-filter: blur(10px);
  border-top: 1px solid var(--color-primary-faint);
  padding: 24px 0;
}

.footer-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
}

.footer-link {
  color: rgba(var(--color-text-rgb), 0.7);
  text-decoration: none;
  font-size: 0.875rem;
  transition: color 0.2s ease;
}

.footer-link:hover {
  color: var(--color-primary);
}

.footer-divider {
  color: rgba(var(--color-text-rgb), 0.3);
  font-size: 0.875rem;
}

.footer-copyright {
  color: rgba(var(--color-text-rgb), 0.5);
  font-size: 0.75rem;
}

@media (max-width: 600px) {
  .footer-links {
    flex-direction: column;
    gap: 8px;
  }

  .footer-divider {
    display: none;
  }
}
</style>
